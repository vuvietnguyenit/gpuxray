// nvmlDevice is a resolved GPU handle with its static metadata attached.
package pid

import (
	"fmt"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
)

type nvmlDevice struct {
	index  int
	uuid   string
	handle nvml.Device
}

// NvmlSession holds an open NVML context and the list of enumerated devices.
// Create one with openNVMLSession(); always call Close() when done.
type NvmlSession struct {
	Devices []nvmlDevice
}

func OpenNVMLSession() (*NvmlSession, error) {
	if ret := nvml.Init(); ret != nvml.SUCCESS {
		return nil, fmt.Errorf("nvml.Init: %s", nvml.ErrorString(ret))
	}

	count, ret := nvml.DeviceGetCount()
	if ret != nvml.SUCCESS {
		nvml.Shutdown() // best-effort; ignore error
		return nil, fmt.Errorf("nvml.DeviceGetCount: %s", nvml.ErrorString(ret))
	}

	devices := make([]nvmlDevice, 0, count)
	for i := range count {
		handle, ret := nvml.DeviceGetHandleByIndex(i)
		if ret != nvml.SUCCESS {
			// Non-fatal: skip this device but continue.
			continue
		}
		uuid, ret := nvml.DeviceGetUUID(handle)
		if ret != nvml.SUCCESS {
			uuid = fmt.Sprintf("gpu%d", i)
		}
		devices = append(devices, nvmlDevice{index: i, uuid: uuid, handle: handle})
	}

	return &NvmlSession{Devices: devices}, nil
}

// Close shuts down the NVML session.
func (s *NvmlSession) Close() {
	nvml.Shutdown()
}

// ---------------------------------------------------------------------------
// Per-device helpers (operate on an open session, no Init/Shutdown)
// ---------------------------------------------------------------------------

// gpuUsageForPID returns a GPUUsage entry for each device the given PID has
// a compute or graphics context on.
func (s *NvmlSession) gpuUsageForPID(pid uint32) []GPUUsage {
	var usages []GPUUsage
	for _, dev := range s.Devices {
		usage, found := dev.usageForPID(pid)
		if found {
			usages = append(usages, usage)
		}
	}
	//
	return usages
}

// AllPIDs returns the deduplicated set of PIDs with an active compute or
// graphics context on any device in this session.
func (s *NvmlSession) AllPIDs() map[uint32]struct{} {
	pids := make(map[uint32]struct{})
	for _, dev := range s.Devices {
		for _, pid := range dev.runningPIDs() {
			pids[pid] = struct{}{}
		}
	}
	return pids
}

// ---------------------------------------------------------------------------
// Per-device helpers (methods on nvmlDevice)
// ---------------------------------------------------------------------------

// runningPIDs returns every PID with an active context on this device.
func (d nvmlDevice) runningPIDs() []uint32 {
	procs, ret := d.handle.GetComputeRunningProcesses()
	if ret != nvml.SUCCESS {
		return nil
	}
	seen := make(map[uint32]struct{}, len(procs))
	pids := make([]uint32, 0, len(procs))
	for _, p := range procs {
		pid := uint32(p.Pid)
		if _, dup := seen[pid]; !dup {
			seen[pid] = struct{}{}
			pids = append(pids, pid)
		}
	}
	return pids
}

// usageForPID returns a populated GPUUsage and true if pid has a context on
// this device, or zero-value and false otherwise.
func (d nvmlDevice) usageForPID(pid uint32) (GPUUsage, bool) {
	procs, ret := d.handle.GetComputeRunningProcesses()
	if ret != nvml.SUCCESS {
		return GPUUsage{}, false
	}

	for _, p := range procs {
		if uint32(p.Pid) != pid {
			continue
		}
		return GPUUsage{
			DeviceIndex:  d.index,
			UUID:         d.uuid,
			Device:       d.handle,
			UsedMemBytes: p.UsedGpuMemory,
		}, true
	}
	return GPUUsage{}, false
}
