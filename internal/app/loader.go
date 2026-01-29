// Declare the function to help operate with eBPF objects
// SPDX-License-Identifier: Apache-2.0
//
// Copyright 2026 Vu Nguyen
package app

import (
	"errors"
	"fmt"
	"log"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/rlimit"
	"github.com/emirpasic/gods/sets/treeset"
	"github.com/vuvietnguyenit/gpuxray/internal"
)

var bpfObjecs bpfObjects

func RemoveMemlock() error {
	if !internal.RemoveMemlock {
		return nil
	}
	// Allow the current process to lock memory for eBPF resources.
	if err := rlimit.RemoveMemlock(); err != nil {
		return fmt.Errorf("failed to remove memlock limit: %w", err)
	}
	log.Println("Removed memlock limit successfully")
	return nil
}

func init() {
	// Load pre-compiled eBPF objects
	bpfObjecs = bpfObjects{}
	if err := loadBpfObjects(&bpfObjecs, nil); err != nil {
		var ve *ebpf.VerifierError
		if errors.As(err, &ve) {
			log.Fatalf("Verifier error: %+v\n", ve)
		}
		log.Fatalf("Failed to load eBPF objects: %v", err)
	}
	fmt.Println("OK: eBPF objects loaded successfully")
}

// Attach probes to the given CUDA shared library.
func attachProbes(cudaLib string, objs *bpfObjects, syms *treeset.Set) {
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
	attach(internal.FilterTreeSetRegex(syms, "cuMemAlloc*"), objs.TraceCudaMallocEntry)
	attachRet(internal.FilterTreeSetRegex(syms, "cuMemAlloc*"), objs.TraceCudaMallocReturn)
	attach(internal.FilterTreeSetRegex(syms, "cuMemFree*"), objs.TraceCudaFree)

	defer func() {
		for _, l := range links {
			l.Close()
		}
	}()
}
