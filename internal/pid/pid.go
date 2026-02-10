// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Vu Nguyen

// This module is created to operate on processes are runnning GPU.
package pid

import (
	"debug/elf"
	"fmt"
	"strings"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"github.com/prometheus/procfs"
	"github.com/shirou/gopsutil/v3/process"
	"github.com/vuvietnguyenit/gpuxray/internal"
	"github.com/vuvietnguyenit/gpuxray/internal/logging"
)

func getCUDASharedObject(pid int) ([]string, error) {
	fs, err := procfs.NewFS("/proc")
	if err != nil {
		return nil, err
	}

	p, err := fs.Proc(pid)
	if err != nil {
		return nil, err
	}

	maps, err := p.ProcMaps()
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(maps))
	for _, lib := range maps {
		out = append(out, lib.Pathname)
	}
	out = internal.FilterValidCUDASharedObjects(out)
	// check shared object path is valid
	for _, lib := range out {
		f, err := internal.CheckFileStat(lib)
		if err != nil {
			continue
		}
		if f.Exists {
			out = append(out, f.Path)
		}
	}
	return out, nil
}

// USProcessInfo defines the structure to hold information about a user-space process using GPU
type USProcessInfo struct {
	PID      uint32
	Comm     string
	Args     string
	CUDALibs []string
}

type GPUUsage struct {
	DeviceIndex int
	UUID        string
}

type PIDInspection struct {
	Process USProcessInfo
	GPUs    []GPUUsage
	Errors  []string
}

type ListProcess []USProcessInfo

// get properties of a process given its PID and device ID. Inpsect to PID to get more infomation of process
func inspectProc(pid uint32) (USProcessInfo, error) {
	p, err := process.NewProcess(int32(pid))
	if err != nil {
		logging.L().Error().
			Int("pid", int(p.Pid)).
			Err(err).
			Msg("failed to get process info")

		return USProcessInfo{}, err
	}
	comm, _ := p.Name()
	args, _ := p.Cmdline()
	cudaLibs, err := getCUDASharedObject(int(pid))
	if err != nil {
		logging.L().Error().
			Int("pid", int(p.Pid)).
			Err(err).
			Msg("failed to get CUDA shared objects")

		return USProcessInfo{}, err
	}
	return USProcessInfo{
		PID:      uint32(pid),
		Comm:     comm,
		Args:     args,
		CUDALibs: cudaLibs,
	}, nil
}

func inspectGPUUsage(pid uint32) ([]GPUUsage, error) {
	var usages []GPUUsage

	ret := nvml.Init()
	if ret != nvml.SUCCESS {
		return nil, fmt.Errorf("nvml.Init: %s", nvml.ErrorString(ret))
	}
	defer nvml.Shutdown()

	count, ret := nvml.DeviceGetCount()
	if ret != nvml.SUCCESS {
		return nil, fmt.Errorf("nvml.DeviceGetCount: %s", nvml.ErrorString(ret))
	}

	for i := range count {
		device, ret := nvml.DeviceGetHandleByIndex(i)
		if ret != nvml.SUCCESS {
			continue
		}

		uuid, _ := device.GetUUID()

		procs, ret := device.GetComputeRunningProcesses()
		if ret != nvml.SUCCESS {
			continue
		}

		for _, p := range procs {
			if p.Pid == pid {
				usages = append(usages, GPUUsage{
					DeviceIndex: i,
					UUID:        uuid,
				})
			}
		}
	}

	return usages, nil
}

func InspectPID(pid uint32) *PIDInspection {
	result := PIDInspection{}

	proc, err := inspectProc(pid)
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
		return &result
	}
	result.Process = proc
	gpus, err := inspectGPUUsage(pid)
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
	}
	result.GPUs = gpus
	return &result
}

// GetRunningProcesses returns all PIDs using CUDA
func GetRunningProcesses() (ListPIDInspection, error) {
	ret := nvml.Init()
	if ret != nvml.SUCCESS {
		return nil, fmt.Errorf("nvml init failed: %s", nvml.ErrorString(ret))
	}
	defer nvml.Shutdown()

	count, ret := nvml.DeviceGetCount()
	if ret != nvml.SUCCESS {
		return nil, fmt.Errorf("DeviceGetCount: %s", nvml.ErrorString(ret))
	}

	gpuPIDs := make(map[uint32]struct{})

	for i := range count {
		dev, ret := nvml.DeviceGetHandleByIndex(i)
		if ret != nvml.SUCCESS {
			logging.L().Error().
				Int("device_index", i).
				Int("nvml_ret", int(ret)).
				Str("nvml_error", nvml.ErrorString(ret)).
				Msg("nvml DeviceGetHandleByIndex failed")

			continue
		}

		procs, ret := dev.GetComputeRunningProcesses()
		if ret != nvml.SUCCESS {
			logging.L().Error().
				Int("nvml_ret", int(ret)).
				Str("nvml_error", nvml.ErrorString(ret)).
				Msg("nvml GetComputeRunningProcesses failed")

			continue
		}

		for _, p := range procs {
			gpuPIDs[p.Pid] = struct{}{}
		}
	}

	results := make([]PIDInspection, 0, len(gpuPIDs))
	for pid := range gpuPIDs {
		results = append(results, *InspectPID(pid))
	}

	return results, nil
}

type ListPIDInspection []PIDInspection

func (pi ListPIDInspection) EnumerateSymNames(prefix string) []string {
	var result []string
	for _, proc := range pi {
		syms, err := EnumerateSym(prefix, proc.Process)
		if err != nil {
			continue
		}
		for _, sym := range syms {
			result = append(result, sym.Name)
		}
	}
	return internal.Deduplicate(result)
}

func (pi ListPIDInspection) CachePID() {
	cache := GlobalPIDCache()
	for _, proc := range pi {
		cache.Set(proc.Process.PID, proc)
	}
}

// Function to enumerate exported APIs from a process's CUDA shared libraries, can provide a prefix
// to enumerate specific APIs related to what CUDA function we want to inspect to.
// For example, prefix = "cuMem" will enumerate all APIs related to Memory Management of CUDA Driver API
// prefix = "cudaMalloc" will enumerate all APIs related to Memory Management of CUDA Runtime API
// prefix = * will enumerate all exported APIs from the CUDA shared libraries

func EnumerateSym(prefix string, p USProcessInfo) ([]elf.Symbol, error) {
	var result []elf.Symbol
	for _, path := range p.CUDALibs {
		syms, err := elf.Open(path)
		if err != nil {
			return nil, err
		}
		defer syms.Close()

		symsList, err := syms.DynamicSymbols()
		if err != nil {
			return nil, err
		}
		if prefix != "*" {
			var filtered []elf.Symbol
			for _, sym := range symsList {
				if strings.HasPrefix(sym.Name, prefix) {
					filtered = append(filtered, sym)
				}
			}
			result = append(result, filtered...)
		} else {
			result = append(result, symsList...)
		}
	}
	return result, nil
}
