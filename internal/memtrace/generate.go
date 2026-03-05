// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Vu Nguyen

package memtrace

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target amd64 -output-dir gen -go-package gen Memdriver ./bpf/memdriver.bpf.c -- -I../../bpf/headers
