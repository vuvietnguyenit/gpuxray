// This module is created to operate on processes are runnning GPU.
// SPDX-License-Identifier: Apache-2.0
//
// Copyright 2026 Vu Nguyen
package app

import (
	"debug/elf"
	"fmt"
	"log"
	"strings"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"github.com/emirpasic/gods/sets/treeset"
	"github.com/prometheus/procfs"
	"github.com/shirou/gopsutil/v3/process"
)

// This variable contains a list of strings that may appear to ignore CUDA's path shared objects
var PATH_STRING_IS_NOT_FROM_CUDA = []string{
	"python",
}

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

	libs := make(map[string]struct{})
	for _, m := range maps {
		if strings.Contains(m.Pathname, "libcuda.so") || strings.Contains(m.Pathname, "libcudart.so") {
			libs[m.Pathname] = struct{}{} // TODO: in future, we can add more information such as address range
			continue
		}
	}
	out := make([]string, 0, len(libs))
	for lib := range libs {
		out = append(out, lib)
	}

	return out, nil
}

type ProcessInfo struct {
	PID        uint32
	Comm       string
	Args       string
	DeviceID   int
	DeviceName string
	CUDALibs   []string
}

type ListProcess []ProcessInfo

// getRunningProcesses returns all PIDs using CUDA
func getRunningProcesses() (ListProcess, error) {

	count, ret := nvml.DeviceGetCount()
	if ret != nvml.SUCCESS {
		return nil, fmt.Errorf("DeviceGetCount: %s", nvml.ErrorString(ret))
	}

	var result []ProcessInfo

	for i := range count {
		dev, ret := nvml.DeviceGetHandleByIndex(i)
		if ret != nvml.SUCCESS {
			continue
		}

		name, _ := dev.GetName()

		procs, ret := dev.GetComputeRunningProcesses()
		if ret != nvml.SUCCESS {
			continue
		}

		for _, proc := range procs {
			p, err := process.NewProcess(int32(proc.Pid))
			if err != nil {
				log.Printf("Failed to get process info for PID %d: %v", p.Pid, err)
				continue
			}
			comm, _ := p.Name()
			args, _ := p.Cmdline()
			cudaLibs, err := getCUDASharedObject(int(proc.Pid))
			if err != nil {
				log.Printf("Failed to get CUDA shared objects for PID %d: %v", p.Pid, err)
				continue
			}
			result = append(result, ProcessInfo{
				PID:        proc.Pid,
				Comm:       comm,
				Args:       args,
				DeviceID:   i,
				DeviceName: name,
				CUDALibs:   cudaLibs,
			})
		}
	}

	return result, nil
}

// Function to check PID is exist in the list of processes it will return ProcessInfo
func (lp ListProcess) findProcessInfoByPID(pid int) *ProcessInfo {
	for _, proc := range lp {
		if int(proc.PID) == pid {
			return &proc
		}
	}
	return nil
}

// Function to scan all shared object paths from a list of ProcessInfo
func (lp ListProcess) getMapLinked(procs []ProcessInfo) *treeset.Set {
	sharedObjectPaths := treeset.NewWithStringComparator()
	for _, proc := range procs {
		for _, lib := range proc.CUDALibs {
			sharedObjectPaths.Add(lib)
		}
	}
	return sharedObjectPaths
}

// Function to enumerate exported APIs from a process's CUDA shared libraries, can provide a prefix
// to enumerate specific APIs related to what CUDA function we want to inspect to.
// For example, prefix = "cuMem" will enumerate all APIs related to Memory Management of CUDA Driver API
// prefix = "cudaMalloc" will enumerate all APIs related to Memory Management of CUDA Runtime API
// prefix = * will enumerate all exported APIs from the CUDA shared libraries

func (p ProcessInfo) enumerateExportedAPIs(prefix string) ([]elf.Symbol, error) {
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

func (lp ListProcess) enumerateExportedAPINames(prefix string) *treeset.Set {
	result := treeset.NewWithStringComparator()
	for _, proc := range lp {
		syms, err := proc.enumerateExportedAPIs(prefix)
		if err != nil {
			continue
		}
		for _, sym := range syms {
			result.Add(sym.Name)
		}
	}
	return result
}
