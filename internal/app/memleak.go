// This module is help to work with memory management of GPU through CUDA APIs
// SPDX-License-Identifier: Apache-2.0
//
// Copyright 2026 Vu Nguyen
package app

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/cilium/ebpf/ringbuf"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cc clang -cflags "-O2 -g -Wall -Werror" -tags linux -target amd64 bpf ../ebpf/memdriver.c -- -I/usr/include/bpf -I.

const (
	EventMalloc        = 0
	EventFree          = 1
	EventMemcpy        = 2
	EventMemset        = 3
	EventMallocManaged = 4
)

// CudaAllocEvent matches the C struct (packed)
type CudaAllocEvent struct {
	Pid       uint32
	Tid       uint32
	Timestamp uint64
	Size      uint64
	DevicePtr uint64
	Comm      [16]byte
	Type      uint32
}

type Stats struct {
	TotalAllocs     int64
	TotalFrees      int64
	TotalMemcpys    int64
	TotalAllocMem   uint64
	TotalFreeMem    uint64
	TotalManagedMem uint64
	ManagedAllocCnt int64
}

func RunMemleakTrace() {
	procs, err := getRunningProcesses() // TODO: we can filter by PID later
	if err != nil {
		log.Printf("Failed to get running processes: %v", err)
	}
	soPath := getSoPaths(procs)
	syms := enumerateSymNames("*", procs)
	for el := soPath.Iterator(); el.Next(); {
		libPath := el.Value().(string)
		fmt.Printf("Attaching probes to CUDA library: %s\n", libPath)
		// Attach uprobes by shared object paths
		attachProbes(libPath, &bpfObjecs, syms)
	}

	// if internal.MemoryleakFlags.Pid != 0 {
	// 	pid := internal.MemoryleakFlags.Pid
	// 	proc := procs.findProcessInfoByPID(int(pid))
	// 	if proc == nil {
	// 		log.Fatalf("PID %d is not using CUDA or does not exist.", pid)
	// 	}
	// 	fmt.Printf("Tracing memory leaks for PID %d (%s)\n", pid, proc.Comm)
	// }

	fmt.Println("eBPF objects loaded successfully")

	// Open ring buffer reader
	rd, err := ringbuf.NewReader(bpfObjecs.CudaEvents)
	if err != nil {
		log.Fatalf("Failed to create ring buffer reader: %v", err)
	}
	defer rd.Close()

	// Handle signals
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	fmt.Println("Tracing CUDA memory operations... Press Ctrl+C to stop.")
	fmt.Println(strings.Repeat("=", 80))

	stats := &Stats{}

	// Read events from ring buffer
	go func() {
		for {
			record, err := rd.Read()
			if err != nil {
				if errors.Is(err, ringbuf.ErrClosed) {
					return
				}
				log.Printf("Error reading from ring buffer: %v", err)
				continue
			}

			handleEvent(record.RawSample, stats)
		}
	}()

	<-ctx.Done()
	fmt.Println("\nStopping tracer...")

	// Print statistics
	printStats(stats)
}

func handleEvent(data []byte, stats *Stats) {
	// Try as alloc event
	var event CudaAllocEvent
	if err := binary.Read(bytes.NewReader(data), binary.LittleEndian, &event); err != nil {
		log.Printf("Failed to parse event: %v", err)
		return
	}

	handleAllocEvent(&event, stats)
}

func handleAllocEvent(event *CudaAllocEvent, stats *Stats) {
	comm := strings.TrimRight(string(event.Comm[:]), "\x00")
	timestamp := time.Unix(0, int64(event.Timestamp))

	switch event.Type {
	case EventMalloc:
		stats.TotalAllocs++
		stats.TotalAllocMem += event.Size
		fmt.Printf("[%s] PID=%d TID=%d ALLOC %s size=%d bytes (%.2f MB)\n",
			timestamp.Format("15:04:05.000000"),
			event.Pid, event.Tid, comm,
			event.Size, float64(event.Size)/(1024*1024))

	case EventMallocManaged:
		stats.TotalAllocs++
		stats.ManagedAllocCnt++
		stats.TotalAllocMem += event.Size
		stats.TotalManagedMem += event.Size
		fmt.Printf("[%s] PID=%d TID=%d ALLOC_MANAGED %s size=%d bytes (%.2f MB)\n",
			timestamp.Format("15:04:05.000000"),
			event.Pid, event.Tid, comm,
			event.Size, float64(event.Size)/(1024*1024))

	case EventFree:
		stats.TotalFrees++
		stats.TotalFreeMem += event.Size
		fmt.Printf("[%s] PID=%d TID=%d FREE %s ptr=0x%x size=%d bytes\n",
			timestamp.Format("15:04:05.000000"),
			event.Pid, event.Tid, comm,
			event.DevicePtr, event.Size)

	case EventMemset:
		fmt.Printf("[%s] PID=%d TID=%d MEMSET %s ptr=0x%x count=%d bytes\n",
			timestamp.Format("15:04:05.000000"),
			event.Pid, event.Tid, comm,
			event.DevicePtr, event.Size)
	}
}

func printStats(stats *Stats) {
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("CUDA Memory Operation Statistics")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("Total Allocations:       %d (%.2f MB)\n",
		stats.TotalAllocs, float64(stats.TotalAllocMem)/(1024*1024))
	fmt.Printf("  - Managed Memory:      %d (%.2f MB)\n",
		stats.ManagedAllocCnt, float64(stats.TotalManagedMem)/(1024*1024))
	fmt.Printf("Total Frees:             %d (%.2f MB)\n",
		stats.TotalFrees, float64(stats.TotalFreeMem)/(1024*1024))
	fmt.Printf("Net Memory (allocated):  %.2f MB\n",
		float64(stats.TotalAllocMem-stats.TotalFreeMem)/(1024*1024))
	fmt.Println(strings.Repeat("=", 80))
}
