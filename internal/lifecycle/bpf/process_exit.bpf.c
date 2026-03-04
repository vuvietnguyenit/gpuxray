// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Vu Nguyen

#include "../../bpf/headers/vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>


char LICENSE[] SEC("license") = "MIT";

/*
 * Global process lifecycle events
 * Stable ABI between kernel and userspace
 */
struct process_exit_event {
    __u32 pid;
    __u32 tgid;
    __u64 exit_ts;
};

/*
 * Global ring buffer for process lifecycle events
 */
struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 1 << 24); /* 16MB */
} lifecycle_events SEC(".maps");

/*
 * Tracepoint: sched:sched_process_exit
 *
 * This is a GLOBAL signal:
 * - consumed by pid cache
 * - used by multiple tracers
 */
SEC("tracepoint/sched/sched_process_exit")
int handle_process_exit(void *ctx)
{
    struct process_exit_event *event;
    __u64 pid_tgid;

    event = bpf_ringbuf_reserve(&lifecycle_events, sizeof(*event), 0);
    if (!event)
        return 0;

    pid_tgid = bpf_get_current_pid_tgid();

    event->pid     = pid_tgid & 0xffffffff;
    event->tgid    = pid_tgid >> 32;
    event->exit_ts = bpf_ktime_get_ns();

    bpf_ringbuf_submit(event, 0);
    return 0;
}
