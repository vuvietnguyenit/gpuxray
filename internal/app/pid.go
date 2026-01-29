// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Vu Nguyen

// This module is created to operate on processes are runnning GPU.
package app

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"github.com/prometheus/procfs"
	"github.com/shirou/gopsutil/v3/process"
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
	PID      uint32
	Comm     string
	Args     string
	DeviceID int
	CUDALibs []string
}

type ListProcess []ProcessInfo

// get properties of a process given its PID and device ID. Inpsect to PID to get more infomation of process
func inspectProc(pid uint32) *ProcessInfo {
	p, err := process.NewProcess(int32(pid))
	if err != nil {
		log.Printf("Failed to get process info for PID %d: %v", p.Pid, err)
		return nil
	}
	comm, _ := p.Name()
	args, _ := p.Cmdline()
	cudaLibs, err := getCUDASharedObject(int(pid))
	if err != nil {
		log.Printf("Failed to get CUDA shared objects for PID %d: %v", p.Pid, err)
		return nil
	}
	return &ProcessInfo{
		PID:      uint32(pid),
		Comm:     comm,
		Args:     args,
		CUDALibs: cudaLibs,
	}
}

// getRunningProcesses returns all PIDs using CUDA
func getRunningProcesses(pid uint32) (ListProcess, error) {
	if pid != 0 && !pidExists(pid) {
		return nil, fmt.Errorf("pid %d does not exist", pid)
	}

	count, ret := nvml.DeviceGetCount()
	if ret != nvml.SUCCESS {
		return nil, fmt.Errorf("DeviceGetCount: %s", nvml.ErrorString(ret))
	}
	var result []ProcessInfo
	found := false
	for i := range count {
		dev, ret := nvml.DeviceGetHandleByIndex(i)
		if ret != nvml.SUCCESS {
			log.Print("DeviceGetHandleByIndex: ", nvml.ErrorString(ret))
			continue
		}
		// step get all processes using this GPU
		procs, ret := dev.GetComputeRunningProcesses()
		if ret != nvml.SUCCESS {
			log.Print("GetComputeRunningProcesses: ", nvml.ErrorString(ret))
			continue
		}
		if pid != 0 {
			if isGPUPidExist(pid, procs) {
				result = append(result, *inspectProc(pid))
				found = true
				break // PID found → stop scanning GPUs
			}
		} else {
			if len(procs) == 0 {
				// No processes using this GPU
				continue
			} else {
				for _, proc := range procs {
					result = append(result, *inspectProc(proc.Pid))
				}
			}
		}

	}
	if pid != 0 && !found {
		return nil, fmt.Errorf("pid %d exists but is not using GPU", pid)
	}

	return result, nil
}

func pidExists(pid uint32) bool {
	_, err := os.Stat(fmt.Sprintf("/proc/%d", pid))
	return err == nil
}

func isGPUPidExist(pid uint32, procs []nvml.ProcessInfo) bool {
	for _, p := range procs {
		if p.Pid == pid {
			return true
		}
	}
	return false
}
