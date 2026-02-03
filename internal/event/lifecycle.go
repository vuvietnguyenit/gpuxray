package event

type LifecycleProcessExitEvent struct {
	Pid    uint32
	Tgid   uint32
	ExitTs uint64
}
