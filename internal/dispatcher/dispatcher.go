// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Vu Nguyen

// internal/dispatcher/dispatcher.go
package dispatcher

import "github.com/vuvietnguyenit/gpuxray/internal/event"

type Handler interface {
	Handle(event.Event)
}

type Dispatcher struct {
	handlers map[event.Type][]Handler
}

func New() *Dispatcher {
	return &Dispatcher{
		handlers: make(map[event.Type][]Handler),
	}
}

func (d *Dispatcher) Register(t event.Type, h Handler) {
	d.handlers[t] = append(d.handlers[t], h)
}

func (d *Dispatcher) Dispatch(e event.Event) {
	for _, h := range d.handlers[e.Type()] {
		h.Handle(e)
	}
}
