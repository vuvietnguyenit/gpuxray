#include "../../bpf/headers/vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>
#include <bpf/bpf_core_read.h>

char LICENSE[] SEC("license") = "MIT";

struct cuinit_event {
    __u32 pid;
    __u32 tgid;
    char  comm[16];
};

struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 1 << 24); // 16MB
} cu_init_events SEC(".maps");

/*
 * Fires when a process calls cuInit()
 */
SEC("uprobe/cuInit")
int uprobe_cuinit(struct pt_regs *ctx)
{
    struct cuinit_event *ev;
    __u64 pid_tgid;

    ev = bpf_ringbuf_reserve(&cu_init_events, sizeof(*ev), 0);
    if (!ev)
        return 0;

    pid_tgid = bpf_get_current_pid_tgid();
    ev->tgid = pid_tgid >> 32;
    ev->pid  = pid_tgid & 0xffffffff;

    bpf_get_current_comm(&ev->comm, sizeof(ev->comm));

    bpf_ringbuf_submit(ev, 0);
    return 0;
}
