// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Vu Nguyen

package lifecycle

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target amd64 -output-dir gen -go-package gen ProcessExit ./bpf/process_exit.bpf.c -- -I../../bpf/headers -D__TARGET_ARCH_x86
