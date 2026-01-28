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
)

var bpfObjecs bpfObjects

func init() {
	if err := rlimit.RemoveMemlock(); err != nil {
		log.Fatalf("Failed to remove memlock limit: %v", err)
	}
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
