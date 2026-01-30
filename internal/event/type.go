package event

type Type uint8

const (
	EventUnknown Type = iota
	EventMemory
	EventCompute
	EventPower
)
