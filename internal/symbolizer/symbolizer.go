// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Vu Nguyen

package symbolizer

import (
	"bufio"
	"debug/elf"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/ianlancetaylor/demangle"
)

type Symbolizer struct {
	mu    sync.RWMutex
	cache map[string]*elfCache
}

type elfCache struct {
	symbols []elf.Symbol
}

func New() *Symbolizer {
	return &Symbolizer{
		cache: make(map[string]*elfCache),
	}
}

func (s *Symbolizer) Resolve(pid int, addr uint64) (string, error) {
	libPath, base, err := findLibraryForAddr(pid, addr)
	if err != nil {
		return "", err
	}

	offset := addr - base

	ec, err := s.loadELF(libPath)
	if err != nil {
		return "", err
	}

	return resolveSymbol(ec.symbols, offset), nil
}
func findLibraryForAddr(pid int, addr uint64) (string, uint64, error) {
	f, err := os.Open(fmt.Sprintf("/proc/%d/maps", pid))
	if err != nil {
		return "", 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		line := scanner.Text()

		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}

		var start, end uint64
		fmt.Sscanf(fields[0], "%x-%x", &start, &end)

		if addr >= start && addr < end {
			return fields[len(fields)-1], start, nil
		}
	}

	return "", 0, fmt.Errorf("address not found in maps")
}
func (s *Symbolizer) loadELF(path string) (*elfCache, error) {
	s.mu.RLock()
	if ec, ok := s.cache[path]; ok {
		s.mu.RUnlock()
		return ec, nil
	}
	s.mu.RUnlock()

	f, err := elf.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	syms, err := f.Symbols()
	if err != nil {
		return nil, err
	}

	sort.Slice(syms, func(i, j int) bool {
		return syms[i].Value < syms[j].Value
	})

	ec := &elfCache{symbols: syms}

	s.mu.Lock()
	s.cache[path] = ec
	s.mu.Unlock()

	return ec, nil
}

func resolveSymbol(symbols []elf.Symbol, offset uint64) string {
	i := sort.Search(len(symbols), func(i int) bool {
		return symbols[i].Value > offset
	})

	if i == 0 {
		return "unknown"
	}

	sym := symbols[i-1]

	if offset >= sym.Value && offset < sym.Value+sym.Size {
		return demangle.Filter(sym.Name)
	}

	return "unknown"
}
