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
	"github.com/cilium/ebpf/rlimit"
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
