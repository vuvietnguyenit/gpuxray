// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Vu Nguyen

package app

import (
	"debug/elf"
	"strings"

	"github.com/emirpasic/gods/sets/treeset"
)

// Function to scan all shared object paths from a list of ProcessInfo
func getSoPaths(procs []ProcessInfo) *treeset.Set {
	sharedObjectPaths := treeset.NewWithStringComparator()
	for _, proc := range procs {
		for _, lib := range proc.CUDALibs {
			sharedObjectPaths.Add(lib)
		}
	}
	return sharedObjectPaths
}

// Function to enumerate exported APIs from a process's CUDA shared libraries, can provide a prefix
// to enumerate specific APIs related to what CUDA function we want to inspect to.
// For example, prefix = "cuMem" will enumerate all APIs related to Memory Management of CUDA Driver API
// prefix = "cudaMalloc" will enumerate all APIs related to Memory Management of CUDA Runtime API
// prefix = * will enumerate all exported APIs from the CUDA shared libraries

func enumerateSym(prefix string, p ProcessInfo) ([]elf.Symbol, error) {
	var result []elf.Symbol
	for _, path := range p.CUDALibs {
		syms, err := elf.Open(path)
		if err != nil {
			return nil, err
		}
		defer syms.Close()

		symsList, err := syms.DynamicSymbols()
		if err != nil {
			return nil, err
		}
		if prefix != "*" {
			var filtered []elf.Symbol
			for _, sym := range symsList {
				if strings.HasPrefix(sym.Name, prefix) {
					filtered = append(filtered, sym)
				}
			}
			result = append(result, filtered...)
		} else {
			result = append(result, symsList...)
		}
	}
	return result, nil
}

func enumerateSymNames(prefix string, lp ListProcess) *treeset.Set {
	result := treeset.NewWithStringComparator()
	for _, proc := range lp {
		syms, err := enumerateSym(prefix, proc)
		if err != nil {
			continue
		}
		for _, sym := range syms {
			result.Add(sym.Name)
		}
	}
	return result
}
