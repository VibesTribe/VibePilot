package runtime

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
)

type AgentPool struct {
	maxPerModule int
	maxTotal     int
	concurrency  *ConcurrencyConfig
	active       atomic.Int32
	perModule    map[string]*int32
	perDest      map[string]*int32
	mu           sync.Mutex
	wg           sync.WaitGroup
	errorCh      chan error
}

func NewAgentPool(maxPerModule, maxTotal int) *AgentPool {
	if maxTotal < maxPerModule {
		maxTotal = maxPerModule
	}
	return &AgentPool{
		maxPerModule: maxPerModule,
		maxTotal:     maxTotal,
		concurrency:  &ConcurrencyConfig{DefaultLimit: maxPerModule},
		perModule:    make(map[string]*int32),
		perDest:      make(map[string]*int32),
		errorCh:      make(chan error, maxTotal),
	}
}

func NewAgentPoolWithConcurrency(maxPerModule, maxTotal int, concurrency *ConcurrencyConfig) *AgentPool {
	if maxTotal < maxPerModule {
		maxTotal = maxPerModule
	}
	if concurrency == nil {
		concurrency = &ConcurrencyConfig{DefaultLimit: maxPerModule}
	}
	return &AgentPool{
		maxPerModule: maxPerModule,
		maxTotal:     maxTotal,
		concurrency:  concurrency,
		perModule:    make(map[string]*int32),
		perDest:      make(map[string]*int32),
		errorCh:      make(chan error, maxTotal),
	}
}

func (p *AgentPool) Submit(ctx context.Context, moduleID string, fn func() error) error {
	return p.SubmitWithDestination(ctx, moduleID, "", fn)
}

func (p *AgentPool) SubmitWithDestination(ctx context.Context, moduleID, destination string, fn func() error) error {
	if !p.acquire(moduleID, destination) {
		return fmt.Errorf("capacity exceeded for module %s or destination %s or total limit reached", moduleID, destination)
	}

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		defer p.release(moduleID, destination)
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[AgentPool] panic recovered in module %s: %v", moduleID, r)
				select {
				case p.errorCh <- fmt.Errorf("panic in %s: %v", moduleID, r):
				default:
				}
			}
		}()

		if err := fn(); err != nil {
			select {
			case p.errorCh <- err:
			default:
				log.Printf("[AgentPool] error channel full, dropping error: %v", err)
			}
		}
	}()

	return nil
}

func (p *AgentPool) acquire(moduleID, destination string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Check total limit
	if int(p.active.Load()) >= p.maxTotal {
		return false
	}

	// Check per-module limit
	moduleCount, ok := p.perModule[moduleID]
	if !ok {
		moduleCount = new(int32)
		p.perModule[moduleID] = moduleCount
	}
	if int(atomic.LoadInt32(moduleCount)) >= p.maxPerModule {
		return false
	}

	// Check per-destination limit
	if destination != "" && p.concurrency != nil {
		destLimit := p.concurrency.GetLimit(destination)
		destCount, ok := p.perDest[destination]
		if !ok {
			destCount = new(int32)
			p.perDest[destination] = destCount
		}
		if int(atomic.LoadInt32(destCount)) >= destLimit {
			return false
		}
		atomic.AddInt32(destCount, 1)
	}

	// All checks passed - acquire
	p.active.Add(1)
	atomic.AddInt32(moduleCount, 1)
	return true
}

func (p *AgentPool) release(moduleID, destination string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if moduleCount, ok := p.perModule[moduleID]; ok {
		atomic.AddInt32(moduleCount, -1)
	}

	if destination != "" {
		if destCount, ok := p.perDest[destination]; ok {
			atomic.AddInt32(destCount, -1)
		}
	}

	p.active.Add(-1)
}

func (p *AgentPool) Wait() {
	p.wg.Wait()
}

func (p *AgentPool) Errors() <-chan error {
	return p.errorCh
}

func (p *AgentPool) DrainErrors() []error {
	var errors []error
	for {
		select {
		case err := <-p.errorCh:
			errors = append(errors, err)
		default:
			return errors
		}
	}
}

func (p *AgentPool) ActiveCount() int {
	return int(p.active.Load())
}

// HasCapacity checks if a slot is available for the given module/destination
// without actually acquiring it. Returns true if a task can be submitted.
func (p *AgentPool) HasCapacity(moduleID, destination string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Check total limit
	if int(p.active.Load()) >= p.maxTotal {
		return false
	}

	// Check per-module limit
	if moduleCount, ok := p.perModule[moduleID]; ok {
		if int(atomic.LoadInt32(moduleCount)) >= p.maxPerModule {
			return false
		}
	}

	// Check per-destination limit
	if destination != "" && p.concurrency != nil {
		destLimit := p.concurrency.GetLimit(destination)
		if destCount, ok := p.perDest[destination]; ok {
			if int(atomic.LoadInt32(destCount)) >= destLimit {
				return false
			}
		}
	}

	return true
}

func (p *AgentPool) ModuleCount(moduleID string) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	if count, ok := p.perModule[moduleID]; ok {
		return int(atomic.LoadInt32(count))
	}
	return 0
}

func (p *AgentPool) Stats() map[string]interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()

	moduleStats := make(map[string]int)
	for key, count := range p.perModule {
		moduleStats[key] = int(atomic.LoadInt32(count))
	}

	destStats := make(map[string]int)
	for key, count := range p.perDest {
		destStats[key] = int(atomic.LoadInt32(count))
	}

	return map[string]interface{}{
		"active_total":   p.ActiveCount(),
		"max_total":      p.maxTotal,
		"max_per_module": p.maxPerModule,
		"modules":        moduleStats,
		"destinations":   destStats,
	}
}
