package internal

type MemleakFlags struct {
	Pid uint32
}

var MemoryleakFlags MemleakFlags
var RemoveMemlock bool
