// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Vu Nguyen

package memtrace

import (
	"fmt"
	"log"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/vuvietnguyenit/gpuxray/internal"
	"github.com/vuvietnguyenit/gpuxray/internal/logging"
	"github.com/vuvietnguyenit/gpuxray/internal/memtrace/gen"
)

type Objects struct {
	*gen.MemdriverObjects
}

func LoadObjects(cfg Config) (*Objects, error) {
	spec, err := gen.LoadMemdriver()
	if err != nil {
		return nil, err
	}
	if err := spec.RewriteConstants(map[string]interface{}{
		"target_pid": cfg.PID,
	}); err != nil {
		return nil, err
	}
	var objs gen.MemdriverObjects
	if err := spec.LoadAndAssign(&objs, nil); err != nil {
		return nil, err
	}
	return &Objects{&objs}, nil
}

func (o *Objects) Close() error {
	return o.MemdriverObjects.Close()
}

// getStack retrieves the stack trace for a given stack ID from the eBPF map.
func (o *Objects) getStack(stackID int32) ([]uint64, error) {
	if stackID < 0 {
		return nil, fmt.Errorf("invalid stack id %d", stackID)
	}
	var stack []uint64 = make([]uint64, 127)

	err := o.StackTraces.Lookup(stackID, &stack)
	if err != nil {
		return nil, err
	}

	// Trim trailing zeroes
	var result []uint64
	for _, addr := range stack {
		if addr == 0 {
			break
		}
		result = append(result, addr)
	}
	return result, nil
}

func AttachProbes(cudaLib string, objs *Objects, syms []string) []link.Link {
	var links []link.Link

	attach := func(symbols []string, prog *ebpf.Program) {
		ex, err := link.OpenExecutable(cudaLib)
		if err != nil {
			log.Fatalf("Failed to open executable %s: %v", cudaLib, err)
		}
		for _, sym := range symbols {
			l, err := ex.Uprobe(sym, prog, nil)
			if err != nil {
				logging.L().Warn().
					Str("probe", "u").
					Str("symbol", sym).
					Err(err).
					Msg("failed to attach probe")
				continue
			}
			links = append(links, l)
			logging.L().Debug().
				Str("probe", "u").
				Str("symbol", sym).
				Msg("attached probe")

		}
	}

	attachRet := func(symbols []string, prog *ebpf.Program) {
		ex, err := link.OpenExecutable(cudaLib)
		if err != nil {
			log.Fatalf("Failed to open executable %s: %v", cudaLib, err)
		}

		for _, sym := range symbols {
			l, err := ex.Uretprobe(sym, prog, nil)
			if err != nil {
				logging.L().Warn().
					Str("probe", "uret").
					Str("symbol", sym).
					Err(err).
					Msg("failed to attach probe")

				continue
			}
			links = append(links, l)
			logging.L().Debug().
				Str("probe", "uret").
				Str("symbol", sym).
				Msg("attached probe")

		}
	}

	// Attach all probes
	attach(internal.FilterSliceRegex(syms, "cuMemAlloc*"), objs.TraceCuMemMallocEntry)
	attachRet(internal.FilterSliceRegex(syms, "cuMemAlloc*"), objs.TraceCuMemMallocReturn)
	attach(internal.FilterSliceRegex(syms, "cuMemFree*"), objs.TraceCuMemFree)
	attach(internal.FilterSliceRegex(syms, "cuMemAllocManaged*"), objs.TraceCuMemMallocManagedEntry)
	attachRet(internal.FilterSliceRegex(syms, "cuMemAllocManaged*"), objs.TraceCuMemMallocManagedReturn)
	return links
}
