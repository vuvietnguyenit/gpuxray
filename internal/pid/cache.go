// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Vu Nguyen

// simplified just create a map and store inspections on it witk key as pid
package pid

import (
	"sync"

	"github.com/vuvietnguyenit/gpuxray/internal"
)

type PIDCache struct {
	mu    sync.RWMutex
	cache map[uint32]PIDInspection
}

func NewPIDCache() *PIDCache {
	return &PIDCache{
		cache: make(map[uint32]PIDInspection),
	}
}

func (c *PIDCache) Get(pid uint32) (PIDInspection, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	ins, ok := c.cache[pid]
	return ins, ok
}

func (c *PIDCache) Set(pid uint32, ins PIDInspection) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache[pid] = ins
}

// This avoids duplicate work when ringbuf fires repeatedly for the same PID.
func (c *PIDCache) GetOrInspect(
	pid uint32,
	inspect func(uint32) (PIDInspection, error),
) PIDInspection {

	// fast path
	if ins, ok := c.Get(pid); ok {
		return ins
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	// double-check after lock

	if ins, ok := c.cache[pid]; ok {
		return ins
	}

	ins, err := inspect(pid)
	if err != nil {
		ins.Errors = append(ins.Errors, err.Error())
	}
	return ins
}

func (c *PIDCache) Delete(pid uint32) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.cache, pid)
}
func (c *PIDCache) Exists(pid uint32) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	_, ok := c.cache[pid]
	return ok
}

func (c *PIDCache) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache = make(map[uint32]PIDInspection)
}

func (c *PIDCache) List() []PIDInspection {
	c.mu.RLock()
	defer c.mu.RUnlock()

	out := make([]PIDInspection, 0, len(c.cache))
	for _, v := range c.cache {
		out = append(out, v)
	}

	return out
}

func (c *PIDCache) GetCUDASharedObjectPaths() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	seen := make(map[string]struct{})

	for _, insp := range c.cache {
		for _, lib := range insp.Process.CUDALibs {
			if lib == "" {
				continue
			}
			seen[lib] = struct{}{}
		}
	}

	out := make([]string, 0, len(seen))
	for lib := range seen {
		f, err := internal.CheckFileStat(lib)
		if err != nil {
			continue
		}
		if f.Exists {
			out = append(out, f.Path)
		}
	}
	return out
}
