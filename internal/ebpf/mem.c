// go:build ignore

#include "vmlinux.h"
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
struct cuda_alloc_event {
  __u32 pid;
  __u32 tid;
  __u64 timestamp;
  __u64 size;
  __u64 device_ptr;
  __u8 comm[TASK_COMM_LEN];
  __u32 type;
} __attribute__((packed));

// Ring buffer for events
struct {
  __uint(type, BPF_MAP_TYPE_RINGBUF);
  __uint(max_entries, 256 * 1024); // 256 KB
} cuda_events SEC(".maps");

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
populate_event_common(struct cuda_alloc_event *event) {
  __u64 pid_tgid = bpf_get_current_pid_tgid();
  event->pid = pid_tgid >> 32;
  event->tid = (__u32)pid_tgid;
  event->timestamp = bpf_ktime_get_ns();
  bpf_get_current_comm(event->comm, TASK_COMM_LEN);
}

SEC("uprobe/cudaMalloc")
int BPF_UPROBE(trace_cuda_malloc_entry, void **devPtr, __u64 size) {
  __u64 pid_tgid = bpf_get_current_pid_tgid();

  struct malloc_info info = {};
  info.size = size;
  info.devptr_addr = (__u64)devPtr;

  bpf_map_update_elem(&malloc_temp, &pid_tgid, &info, BPF_ANY);

  return 0;
}

// cudaMalloc return
SEC("uretprobe/cudaMalloc")
int BPF_URETPROBE(trace_cuda_malloc_return, int ret) {
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
  struct cuda_alloc_event *event =
      bpf_ringbuf_reserve(&cuda_events, sizeof(*event), 0);
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

// cudaFree: void *devPtr
SEC("uprobe/cudaFree")
int BPF_UPROBE(trace_cuda_free, void *devPtr) {
  __u64 device_ptr = (__u64)devPtr;

  struct cuda_alloc_event *event =
      bpf_ringbuf_reserve(&cuda_events, sizeof(*event), 0);
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

// cudaMallocManaged: void **devPtr, size_t size, unsigned int flags
SEC("uprobe/cudaMallocManaged")
int BPF_UPROBE(trace_cuda_malloc_managed_entry, void **devPtr, __u64 size,
               unsigned int flags) {
  __u64 pid_tgid = bpf_get_current_pid_tgid();

  struct malloc_info info = {};
  info.size = size;
  info.devptr_addr = (__u64)devPtr;

  bpf_map_update_elem(&malloc_temp, &pid_tgid, &info, BPF_ANY);

  return 0;
}

SEC("uretprobe/cudaMallocManaged")
int BPF_URETPROBE(trace_cuda_malloc_managed_return, int ret) {
  if (ret != 0)
    return 0;

  __u64 pid_tgid = bpf_get_current_pid_tgid();
  struct malloc_info *info = bpf_map_lookup_elem(&malloc_temp, &pid_tgid);
  if (!info)
    return 0;

  void *devptr_ptr = (void *)info->devptr_addr;
  __u64 device_ptr = 0;
  bpf_probe_read_user(&device_ptr, sizeof(device_ptr), devptr_ptr);

  struct cuda_alloc_event *event =
      bpf_ringbuf_reserve(&cuda_events, sizeof(*event), 0);
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

// cudaMemset: void *devPtr, int value, size_t count
SEC("uprobe/cudaMemset")
int BPF_UPROBE(trace_cuda_memset, void *devPtr, int value, __u64 count) {
  struct cuda_alloc_event *event =
      bpf_ringbuf_reserve(&cuda_events, sizeof(*event), 0);
  if (!event)
    return 0;

  populate_event_common(event);
  event->device_ptr = (__u64)devPtr;
  event->size = count;
  event->type = EVENT_MEMSET;

  bpf_ringbuf_submit(event, 0);

  return 0;
}

char LICENSE[] SEC("license") = "GPL";