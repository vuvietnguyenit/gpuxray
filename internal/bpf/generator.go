// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Vu Nguyen

package bpf

//go:generate sh -c "bpftool btf dump file /sys/kernel/btf/vmlinux format c > headers/vmlinux.h"
