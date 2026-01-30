

## GUI

```sh
PID   USER   GPU   LEAK(MB)  SM%   ENC%   DEC%   API     CMD
1234  vu     0     2048      80    0      0      CUDA    python train.py
4321  root   1     512       10    20     0      NVDEC  ffmpeg ...

```


I have a struct:

type MemoryAllocEvent struct {
	Pid       uint32
	Tid       uint32
	Timestamp uint64
    DeviceID  int8
	Size      uint64
	DevicePtr uint64
	Comm      [16]byte
	Type      uint32
}
Need to build a function, it will take this event, groupby by (Pid, Tid, DeviceID)
