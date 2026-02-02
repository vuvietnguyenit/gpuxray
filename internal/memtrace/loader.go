// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Vu Nguyen

package memtrace

import (
	"fmt"
	"log"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/vuvietnguyenit/gpuxray/internal"
	"github.com/vuvietnguyenit/gpuxray/internal/memtrace/gen"
)

type Objects struct {
	*gen.MemdriverObjects
}

func LoadObjects(cfg Config) (*Objects, error) {
	var objs gen.MemdriverObjects

	if err := gen.LoadMemdriverObjects(&objs, nil); err != nil {
		return nil, err
	}

	// optional: PID / device filtering via maps here

	return &Objects{&objs}, nil
}

func (o *Objects) Close() error {
	return o.MemdriverObjects.Close()
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
				log.Printf("Warning: Failed to attach uprobe to %s: %v", sym, err)
				continue
			}
			links = append(links, l)
			fmt.Printf("✓ Attached uprobe to %s\n", sym)
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
				log.Printf("Warning: Failed to attach uretprobe to %s: %v", sym, err)
				continue
			}
			links = append(links, l)
			fmt.Printf("✓ Attached uretprobe to %s\n", sym)
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
