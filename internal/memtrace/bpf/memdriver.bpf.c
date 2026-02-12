// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Vu Nguyen

// go:build ignore
//  +build ignore

#include "../../bpf/headers/vmlinux.h"
#include <bpf/bpf_core_read.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>

#define TASK_COMM_LEN 16
#define PERF_MAX_STACK_DEPTH 127
const volatile u32 target_pid = 0;

enum event_type {
  EVENT_MALLOC = 0,
  EVENT_FREE = 1,
  EVENT_MEMCPY = 2,
  EVENT_MEMSET = 3,
  EVENT_MALLOC_MANAGED = 4,
};

/* =========================
 * Stack trace map
 * ========================= */
struct {
  __uint(type, BPF_MAP_TYPE_STACK_TRACE);
  __uint(max_entries, 16384);
  __uint(key_size, sizeof(u32));
  __uint(value_size, sizeof(u64) * PERF_MAX_STACK_DEPTH);
} stack_traces SEC(".maps");

/* =========================
 * Event struct
 * ========================= */
struct cu_mem_alloc_event {
  u32 pid;
  u32 tid;
  u32 type;
  s32 stack_id;

  u64 timestamp;
  u64 size;
  u64 device_ptr;
};

/* =========================
 * Ring buffer
 * ========================= */
struct {
  __uint(type, BPF_MAP_TYPE_RINGBUF);
  __uint(max_entries, 1 << 24);
} memleak_ringbuf_events SEC(".maps");

/* =========================
 * Active allocations map
 * ========================= */
struct {
  __uint(type, BPF_MAP_TYPE_HASH);
  __uint(max_entries, 10240);
  __type(key, u64);
  __type(value, u64);
} active_allocs SEC(".maps");

/* =========================
 * Temp malloc storage
 * ========================= */
struct malloc_info {
  u64 size;
  u64 devptr_addr;
};

struct {
  __uint(type, BPF_MAP_TYPE_HASH);
  __uint(max_entries, 1024);
  __type(key, u64);
  __type(value, struct malloc_info);
} malloc_temp SEC(".maps");

/* =========================
 * Helpers
 * ========================= */
static __always_inline int should_trace(void) {
  if (target_pid == 0)
    return 1; // trace all if not set (optional behavior)

  u64 pid_tgid = bpf_get_current_pid_tgid();
  u32 pid = pid_tgid >> 32;

  return pid == target_pid;
}

static __always_inline void
populate_event_common(struct cu_mem_alloc_event *event) {
  u64 pid_tgid = bpf_get_current_pid_tgid();
  event->pid = pid_tgid >> 32;
  event->tid = (u32)pid_tgid;
  event->timestamp = bpf_ktime_get_ns();
}

/* =========================
 * cuMemAlloc
 * ========================= */
SEC("uprobe/cuMemAlloc")
int BPF_UPROBE(trace_cu_mem_malloc_entry, void **devPtr, u64 size) {
  if (!should_trace())
    return 0;

  u64 pid_tgid = bpf_get_current_pid_tgid();

  struct malloc_info info = {};
  info.size = size;
  info.devptr_addr = (u64)devPtr;

  bpf_map_update_elem(&malloc_temp, &pid_tgid, &info, BPF_ANY);
  return 0;
}

SEC("uretprobe/cuMemAlloc")
int BPF_URETPROBE(trace_cu_mem_malloc_return, int ret) {
  if (ret != 0)
    return 0;

  if (!should_trace())
    return 0;

  u64 pid_tgid = bpf_get_current_pid_tgid();
  struct malloc_info *info = bpf_map_lookup_elem(&malloc_temp, &pid_tgid);
  if (!info)
    return 0;

  u64 device_ptr = 0;
  bpf_probe_read_user(&device_ptr, sizeof(device_ptr),
                      (void *)info->devptr_addr);
  s32 stid = 0;
  stid = bpf_get_stackid(ctx, &stack_traces, BPF_F_USER_STACK);

  struct cu_mem_alloc_event *event =
      bpf_ringbuf_reserve(&memleak_ringbuf_events, sizeof(*event), 0);
  if (!event)
    goto cleanup;

  populate_event_common(event);
  event->size = info->size;
  event->device_ptr = device_ptr;
  event->type = EVENT_MALLOC;
  event->stack_id = stid;
  bpf_ringbuf_submit(event, 0);

  if (device_ptr)
    bpf_map_update_elem(&active_allocs, &device_ptr, &info->size, BPF_ANY);

cleanup:
  bpf_map_delete_elem(&malloc_temp, &pid_tgid);
  return 0;
}

/* =========================
 * cuMemFree
 * ========================= */
SEC("uprobe/cuMemFree")
int BPF_UPROBE(trace_cu_mem_free, void *devPtr) {
  if (!should_trace())
    return 0;

  u64 device_ptr = (u64)devPtr;

  struct cu_mem_alloc_event *event =
      bpf_ringbuf_reserve(&memleak_ringbuf_events, sizeof(*event), 0);
  if (!event)
    return 0;

  populate_event_common(event);
  event->device_ptr = device_ptr;
  event->type = EVENT_FREE;
  event->stack_id = bpf_get_stackid(ctx, &stack_traces, BPF_F_USER_STACK);

  u64 *size = bpf_map_lookup_elem(&active_allocs, &device_ptr);
  if (size) {
    event->size = *size;
    bpf_map_delete_elem(&active_allocs, &device_ptr);
  } else {
    event->size = 0;
  }

  bpf_ringbuf_submit(event, 0);
  return 0;
}

/* =========================
 * cuMemAllocManaged
 * ========================= */
SEC("uprobe/cuMemAllocManaged")
int BPF_UPROBE(trace_cu_mem_malloc_managed_entry, void **devPtr, u64 size,
               unsigned int flags) {
  if (!should_trace())
    return 0;

  u64 pid_tgid = bpf_get_current_pid_tgid();

  struct malloc_info info = {};
  info.size = size;
  info.devptr_addr = (u64)devPtr;

  bpf_map_update_elem(&malloc_temp, &pid_tgid, &info, BPF_ANY);
  return 0;
}

SEC("uretprobe/cuMemAllocManaged")
int BPF_URETPROBE(trace_cu_mem_malloc_managed_return, int ret) {
  if (ret != 0)
    return 0;

  if (!should_trace())
    return 0;

  u64 pid_tgid = bpf_get_current_pid_tgid();
  struct malloc_info *info = bpf_map_lookup_elem(&malloc_temp, &pid_tgid);
  if (!info)
    return 0;

  u64 device_ptr = 0;
  bpf_probe_read_user(&device_ptr, sizeof(device_ptr),
                      (void *)info->devptr_addr);

  struct cu_mem_alloc_event *event =
      bpf_ringbuf_reserve(&memleak_ringbuf_events, sizeof(*event), 0);
  if (!event)
    goto cleanup;

  populate_event_common(event);
  event->size = info->size;
  event->device_ptr = device_ptr;
  event->type = EVENT_MALLOC_MANAGED;
  event->stack_id = bpf_get_stackid(ctx, &stack_traces, BPF_F_USER_STACK);

  bpf_ringbuf_submit(event, 0);

  if (device_ptr)
    bpf_map_update_elem(&active_allocs, &device_ptr, &info->size, BPF_ANY);

cleanup:
  bpf_map_delete_elem(&malloc_temp, &pid_tgid);
  return 0;
}

char LICENSE[] SEC("license") = "GPL";
