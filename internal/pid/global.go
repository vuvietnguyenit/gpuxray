package pid

import "sync"

var (
	globalCache  *PIDCache
	once         sync.Once
	globalSoPath []string
)

func GlobalPIDCache() *PIDCache {
	// singleton pattern with lazy initialization
	once.Do(func() {
		globalCache = NewPIDCache()
	})
	return globalCache
}

// func GlobalSoPaths() []string {
// 	once.Do(func() {
// 		globalSoPath = enumerateSystemCUDALibs()
// 	})
// 	return globalSoPath
// }
