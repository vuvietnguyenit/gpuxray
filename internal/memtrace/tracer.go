// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Vu Nguyen

package memtrace

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/cilium/ebpf/ringbuf"
)

type Tracer struct {
	rd *ringbuf.Reader
}

func NewTracer(rd *ringbuf.Reader) *Tracer {
	return &Tracer{rd: rd}
}

func NewRingbufReader(objs *Objects) (*ringbuf.Reader, error) {
	return ringbuf.NewReader(objs.MemleakRingbufEvents)
}

func (t *Tracer) Run(ctx context.Context) error {
	go func() {
		<-ctx.Done()
		t.rd.Close()
		fmt.Println("mem trace stopped")
	}()
	for {
		select {
		case <-ctx.Done():
			return nil

		default:
			record, err := t.rd.Read()
			if err != nil {
				if errors.Is(err, ringbuf.ErrClosed) {
					return nil
				}
				log.Printf("ringbuf read error: %v", err)
				continue
			}

			ev, err := decodeMemoryEvent(record.RawSample)
			if err != nil {
				log.Printf("decode error: %v", err)
				continue
			}

			log.Printf("Memory Event: %+v", ev)
		}
	}
}
