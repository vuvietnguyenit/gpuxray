package bpf

//go:generate sh -c "bpftool btf dump file /sys/kernel/btf/vmlinux format c > headers/vmlinux.h"
