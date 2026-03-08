// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Vu Nguyen

// This module is created to operate on processes are runnning GPU.
package pid

import (
	"debug/elf"
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
	Username string
	CUDALibs []string
}

type GPUUsage struct {
	DeviceIndex  int
	UUID         string
	Device       nvml.Device
	UsedMemBytes uint64 // bytes of GPU VRAM consumed
	SMUtil       uint32 // SM utilisation 0-100 (driver ≥ r470; else 0)
}

type PIDInspection struct {
	Process USProcessInfo
	GPUs    []GPUUsage
	Errors  []string
}

type ListProcess []USProcessInfo

// getProcInfo retrieves properties of a process given its PID. It inspects the PID
// to get process information such as name and command line arguments.
// If resolveCUDA is true, it will also resolve any CUDA shared objects linked to the process.
func getProcInfo(pid uint32, resolveCUDA bool) (USProcessInfo, error) {
	p, err := process.NewProcess(int32(pid))
	if err != nil {
		return USProcessInfo{}, err
	}

	comm, _ := p.Name()
	args, _ := p.Cmdline()
	username, _ := p.Username()

	var cudaLibs []string
	if resolveCUDA {
		cudaLibs, err = getCUDASharedObject(int(pid))
		if err != nil {
			return USProcessInfo{}, err
		}
	}
	return USProcessInfo{
		PID:      uint32(pid),
		Comm:     comm,
		Args:     args,
		Username: username,
		CUDALibs: cudaLibs,
	}, nil
}

func InspectPID(pid uint32) *PIDInspection {
	result := PIDInspection{}

	proc, err := getProcInfo(pid, true)
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
		return &result
	}
	result.Process = proc
	sess, err := OpenNVMLSession()
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
		return &result
	}
	defer sess.Close()
	result.GPUs = sess.gpuUsageForPID(pid)
	return &result
}

// GetRunningProcesses returns an inspection for every PID currently using a GPU.
//
// When sess is non-nil the provided session is reused (lastSeen advances across
// calls — correct for the long-lived Collector use-case).
// When sess is nil a temporary session is opened and closed internally
// (suitable for one-shot CLI / diagnostic calls).
func GetRunningProcesses(sess *NvmlSession) (ListPIDInspection, error) {
	pids := sess.AllPIDs()
	results := make(ListPIDInspection, 0, len(pids))
	for pid := range pids {
		insp := PIDInspection{
			GPUs: sess.gpuUsageForPID(pid),
		}

		proc, err := getProcInfo(pid, true)
		if err != nil {
			insp.Errors = append(insp.Errors, err.Error())
		} else {
			insp.Process = proc
		}

		results = append(results, insp)
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
