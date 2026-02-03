// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Vu Nguyen

// This module is created to operate on processes are runnning GPU.
package pid

import (
	"debug/elf"
	"fmt"
	"log"
	"strings"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"github.com/prometheus/procfs"
	"github.com/shirou/gopsutil/v3/process"
	"github.com/vuvietnguyenit/gpuxray/internal"
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
		log.Printf("Failed to get process info for PID %d: %v", p.Pid, err)
		return USProcessInfo{}, err
	}
	comm, _ := p.Name()
	args, _ := p.Cmdline()
	cudaLibs, err := getCUDASharedObject(int(pid))
	if err != nil {
		log.Printf("Failed to get CUDA shared objects for PID %d: %v", p.Pid, err)
		return USProcessInfo{}, err
	}
	return USProcessInfo{
		PID:      uint32(pid),
		Comm:     comm,
		Args:     args,
		CUDALibs: cudaLibs,
	}, nil
}

func inspectGPUUsage(pid int32) ([]GPUUsage, error) {
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
			if int32(p.Pid) == pid {
				usages = append(usages, GPUUsage{
					DeviceIndex: i,
					UUID:        uuid,
				})
			}
		}
	}

	return usages, nil
}

func InspectPID(pid int32) PIDInspection {
	result := PIDInspection{}

	proc, err := inspectProc(uint32(pid))
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
		return result
	}
	result.Process = proc
	gpus, err := inspectGPUUsage(pid)
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
	}
	result.GPUs = gpus

	return result
}

// GetRunningProcesses returns all PIDs using CUDA
func GetRunningProcesses() ([]PIDInspection, error) {
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
			log.Printf("DeviceGetHandleByIndex(%d): %s", i, nvml.ErrorString(ret))
			continue
		}

		procs, ret := dev.GetComputeRunningProcesses()
		if ret != nvml.SUCCESS {
			log.Printf("GetComputeRunningProcesses: %s", nvml.ErrorString(ret))
			continue
		}

		for _, p := range procs {
			gpuPIDs[p.Pid] = struct{}{}
		}
	}

	results := make([]PIDInspection, 0, len(gpuPIDs))
	for pid := range gpuPIDs {
		results = append(results, InspectPID(int32(pid)))
	}

	return results, nil
}

type ListPIDInspection []PIDInspection

// Function to scan all shared object paths from a list of PIDInspection
func (pi ListPIDInspection) GetSoPaths() []string {
	var sharedObjectPaths []string
	for _, proc := range pi {
		for _, lib := range proc.Process.CUDALibs {
			sharedObjectPaths = append(sharedObjectPaths, lib)
		}
	}
	sharedObjectPaths = internal.FilterValidCUDASharedObjects(sharedObjectPaths)
	return sharedObjectPaths
}

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
	cache := Global()
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
