package pid

import "sync"

var (
	globalCache *PIDCache
	once        sync.Once
)

func Global() *PIDCache {
	// singleton pattern with lazy initialization
	once.Do(func() {
		globalCache = NewPIDCache()
	})
	return globalCache
}
