package throttle

import (
	"sync"
)

type ModuleLimiter struct {
	maxPerModule int
	active       map[string]int
	mu           sync.RWMutex
}

func NewModuleLimiter(maxPerModule int) *ModuleLimiter {
	return &ModuleLimiter{
		maxPerModule: maxPerModule,
		active:       make(map[string]int),
	}
}

func (l *ModuleLimiter) CanDispatch(sliceID string) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if sliceID == "" {
		return true
	}
	return l.active[sliceID] < l.maxPerModule
}

func (l *ModuleLimiter) Acquire(sliceID string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if sliceID == "" {
		return true
	}
	if l.active[sliceID] >= l.maxPerModule {
		return false
	}
	l.active[sliceID]++
	return true
}

func (l *ModuleLimiter) Release(sliceID string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if sliceID == "" {
		return
	}
	if l.active[sliceID] > 0 {
		l.active[sliceID]--
	}
}

func (l *ModuleLimiter) Count(sliceID string) int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.active[sliceID]
}

func (l *ModuleLimiter) Counts() map[string]int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	result := make(map[string]int)
	for k, v := range l.active {
		result[k] = v
	}
	return result
}
