// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Vu Nguyen

package internal

type MemleakFlags struct {
	Pid        uint32
	PrintStack bool
	DeviceID   int
}

var MemoryleakFlags MemleakFlags
var RemoveMemlock bool
var FetchInterval int
var CudaSo string
var LogLevel string
var LogFormat string
