package so

import (
	"debug/elf"
	"fmt"
	"sync"
)

var (
	loadOnce sync.Once
	loadErr  error

	symbols = make(map[string]struct{})
)

func InitFromSharedObject(path string) error {
	loadOnce.Do(func() {
		loadErr = loadSymbols(path)
	})
	return loadErr
}

func loadSymbols(path string) error {
	if path == "" {
		return fmt.Errorf("cuda shared object path is empty")
	}

	f, err := elf.Open(path)
	if err != nil {
		return fmt.Errorf("open ELF failed: %w", err)
	}
	defer f.Close()

	syms, err := f.DynamicSymbols()
	if err != nil {
		return fmt.Errorf("read dynamic symbols failed: %w", err)
	}
	for _, s := range syms {
		if s.Name != "" {
			symbols[s.Name] = struct{}{}
		}
	}

	return nil
}

func HasSymbol(name string) bool {
	_, ok := symbols[name]
	return ok
}

func ListSymbols() []string {
	out := make([]string, 0, len(symbols))
	for s := range symbols {
		out = append(out, s)
	}
	return out
}
