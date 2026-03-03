package pid

import "sync"

var (
	globalCache *PIDCache
	once        sync.Once
)

func GlobalPIDCache() *PIDCache {
	// singleton pattern with lazy initialization
	once.Do(func() {
		globalCache = NewPIDCache()
	})
	return globalCache
}
