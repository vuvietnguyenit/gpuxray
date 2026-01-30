// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Vu Nguyen

// go:build ignore
// +build ignore

#include "../../bpf/headers/vmlinux.h"
#include <bpf/bpf_core_read.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>

// Character array size for process name
#define TASK_COMM_LEN 16

// Event types
enum event_type {
  EVENT_MALLOC = 0,
  EVENT_FREE = 1,
  EVENT_MEMCPY = 2,
  EVENT_MEMSET = 3,
  EVENT_MALLOC_MANAGED = 4,
};

// Structure for allocation/free events
struct cu_mem_alloc_event {
  __u32 pid;
  __u32 tid;
  __u64 timestamp;
  __u64 size;
  __u64 device_ptr;
  __u32 type;
} __attribute__((packed));

// Ring buffer for memory-leaked events
struct {
  __uint(type, BPF_MAP_TYPE_RINGBUF);
  __uint(max_entries, 256 * 1024); // 256 KB
} memleak_ringbuf_events SEC(".maps");

// Hash map to track allocations (device_ptr -> size)
struct {
  __uint(type, BPF_MAP_TYPE_HASH);
  __uint(max_entries, 10240);
  __type(key, __u64);
  __type(value, __u64);
} active_allocs SEC(".maps");

// Temporary storage for malloc entry (to get size before return)
struct malloc_info {
  __u64 size;
  __u64 devptr_addr;
};

struct {
  __uint(type, BPF_MAP_TYPE_HASH);
  __uint(max_entries, 1024);
  __type(key, __u64);
  __type(value, struct malloc_info);
} malloc_temp SEC(".maps");

// Helper to populate common event fields
static __always_inline void
populate_event_common(struct cu_mem_alloc_event *event) {
  __u64 pid_tgid = bpf_get_current_pid_tgid();
  event->pid = pid_tgid >> 32;
  event->tid = (__u32)pid_tgid;
  event->timestamp = bpf_ktime_get_ns();
}

SEC("uprobe/cuMemAlloc")
int BPF_UPROBE(trace_cu_mem_malloc_entry, void **devPtr, __u64 size) {
  __u64 pid_tgid = bpf_get_current_pid_tgid();

  struct malloc_info info = {};
  info.size = size;
  info.devptr_addr = (__u64)devPtr;

  bpf_map_update_elem(&malloc_temp, &pid_tgid, &info, BPF_ANY);

  return 0;
}

// cuMemAlloc return
SEC("uretprobe/cuMemAlloc")
int BPF_URETPROBE(trace_cu_mem_malloc_return, int ret) {
  if (ret != 0) // cudaSuccess = 0
    return 0;

  __u64 pid_tgid = bpf_get_current_pid_tgid();
  struct malloc_info *info = bpf_map_lookup_elem(&malloc_temp, &pid_tgid);
  if (!info)
    return 0;

  // Try to read the device pointer that was written
  void *devptr_ptr = (void *)info->devptr_addr;
  __u64 device_ptr = 0;
  bpf_probe_read_user(&device_ptr, sizeof(device_ptr), devptr_ptr);

  // Reserve space in ring buffer
  struct cu_mem_alloc_event *event =
      bpf_ringbuf_reserve(&memleak_ringbuf_events, sizeof(*event), 0);
  if (!event)
    goto cleanup;

  populate_event_common(event);
  event->size = info->size;
  event->device_ptr = device_ptr;
  event->type = EVENT_MALLOC;

  bpf_ringbuf_submit(event, 0);

  // Track allocation
  if (device_ptr != 0) {
    bpf_map_update_elem(&active_allocs, &device_ptr, &info->size, BPF_ANY);
  }

cleanup:
  bpf_map_delete_elem(&malloc_temp, &pid_tgid);
  return 0;
}

// cuMemFree: void *devPtr
SEC("uprobe/cuMemFree")
int BPF_UPROBE(trace_cu_mem_free, void *devPtr) {
  __u64 device_ptr = (__u64)devPtr;

  struct cu_mem_alloc_event *event =
      bpf_ringbuf_reserve(&memleak_ringbuf_events, sizeof(*event), 0);
  if (!event)
    return 0;

  populate_event_common(event);
  event->device_ptr = device_ptr;
  event->type = EVENT_FREE;

  // Look up size from tracking map
  __u64 *size = bpf_map_lookup_elem(&active_allocs, &device_ptr);
  if (size) {
    event->size = *size;
    bpf_map_delete_elem(&active_allocs, &device_ptr);
  } else {
    event->size = 0;
  }

  bpf_ringbuf_submit(event, 0);

  return 0;
}

// cuMemAllocManaged: void **devPtr, size_t size, unsigned int flags
SEC("uprobe/cuMemAllocManaged")
int BPF_UPROBE(trace_cu_mem_malloc_managed_entry, void **devPtr, __u64 size,
               unsigned int flags) {
  __u64 pid_tgid = bpf_get_current_pid_tgid();

  struct malloc_info info = {};
  info.size = size;
  info.devptr_addr = (__u64)devPtr;

  bpf_map_update_elem(&malloc_temp, &pid_tgid, &info, BPF_ANY);

  return 0;
}

SEC("uretprobe/cuMemAllocManaged")
int BPF_URETPROBE(trace_cu_mem_malloc_managed_return, int ret) {
  if (ret != 0)
    return 0;

  __u64 pid_tgid = bpf_get_current_pid_tgid();
  struct malloc_info *info = bpf_map_lookup_elem(&malloc_temp, &pid_tgid);
  if (!info)
    return 0;

  void *devptr_ptr = (void *)info->devptr_addr;
  __u64 device_ptr = 0;
  bpf_probe_read_user(&device_ptr, sizeof(device_ptr), devptr_ptr);

  struct cu_mem_alloc_event *event =
      bpf_ringbuf_reserve(&memleak_ringbuf_events, sizeof(*event), 0);
  if (!event)
    goto cleanup;

  populate_event_common(event);
  event->size = info->size;
  event->device_ptr = device_ptr;
  event->type = EVENT_MALLOC_MANAGED;

  bpf_ringbuf_submit(event, 0);

  if (device_ptr != 0) {
    bpf_map_update_elem(&active_allocs, &device_ptr, &info->size, BPF_ANY);
  }

cleanup:
  bpf_map_delete_elem(&malloc_temp, &pid_tgid);
  return 0;
}

char LICENSE[] SEC("license") = "GPL";