package app

import (
	"fmt"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
)

type ProcessInfo struct {
	PID        uint32
	DeviceID   int
	DeviceName string
	CUDALibs   []string
}

// GetRunningProcesses returns all PIDs using CUDA
func GetRunningProcesses() ([]ProcessInfo, error) {

	count, ret := nvml.DeviceGetCount()
	if ret != nvml.SUCCESS {
		return nil, fmt.Errorf("DeviceGetCount: %s", nvml.ErrorString(ret))
	}

	var result []ProcessInfo

	for i := 0; i < count; i++ {
		dev, ret := nvml.DeviceGetHandleByIndex(i)
		if ret != nvml.SUCCESS {
			continue
		}

		name, _ := dev.GetName()

		procs, ret := dev.GetComputeRunningProcesses()
		if ret != nvml.SUCCESS {
			continue
		}

		for _, p := range procs {
			result = append(result, ProcessInfo{
				PID:        p.Pid,
				DeviceID:   i,
				DeviceName: name,
			})
		}
	}

	return result, nil
}
