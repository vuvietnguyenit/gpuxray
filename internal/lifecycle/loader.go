// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Vu Nguyen

package lifecycle

import (
	"fmt"

	"github.com/cilium/ebpf/link"
	"github.com/vuvietnguyenit/gpuxray/internal/lifecycle/gen"
)

type Objects struct {
	*gen.ProcessExitObjects
}

func LoadObjects(cfg Config) (*Objects, error) {
	var objs gen.ProcessExitObjects
	if err := gen.LoadProcessExitObjects(&objs, nil); err != nil {
		return nil, err
	}
	return &Objects{&objs}, nil
}

func (o *Objects) Close() error {
	return o.ProcessExitObjects.Close()
}

func attactTracepoints(objs *Objects, cfg Config) error {
	_, err := link.Tracepoint(
		"sched",
		"sched_process_exit",
		objs.HandleProcessExit,
		nil,
	)
	if err != nil {
		objs.Close()
		return fmt.Errorf("attach tracepoint: %w", err)
	}
	return nil
}
