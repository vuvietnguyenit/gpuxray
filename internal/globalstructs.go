// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Vu Nguyen

package internal

type MemleakFlags struct {
	Pid uint32
}

var MemoryleakFlags MemleakFlags
var RemoveMemlock bool
