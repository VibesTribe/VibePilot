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
	perModule    sync.Map
	perDest      sync.Map
	sem          chan struct{}
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
		sem:          make(chan struct{}, maxTotal),
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
		sem:          make(chan struct{}, maxTotal),
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
	currentTotal := p.active.Load()
	if int(currentTotal) >= p.maxTotal {
		return false
	}

	moduleCountI, _ := p.perModule.LoadOrStore(moduleID, new(int32))
	moduleCount := moduleCountI.(*int32)

	if int(atomic.LoadInt32(moduleCount)) >= p.maxPerModule {
		return false
	}

	if destination != "" && p.concurrency != nil {
		destLimit := p.concurrency.GetLimit(destination)
		destCountI, _ := p.perDest.LoadOrStore(destination, new(int32))
		destCount := destCountI.(*int32)

		if int(atomic.LoadInt32(destCount)) >= destLimit {
			return false
		}

		select {
		case p.sem <- struct{}{}:
			p.active.Add(1)
			atomic.AddInt32(moduleCount, 1)
			atomic.AddInt32(destCount, 1)
			return true
		default:
			return false
		}
	}

	select {
	case p.sem <- struct{}{}:
		p.active.Add(1)
		atomic.AddInt32(moduleCount, 1)
		return true
	default:
		return false
	}
}

func (p *AgentPool) release(moduleID, destination string) {
	moduleCountI, ok := p.perModule.Load(moduleID)
	if ok {
		atomic.AddInt32(moduleCountI.(*int32), -1)
	}

	if destination != "" {
		if destCountI, ok := p.perDest.Load(destination); ok {
			atomic.AddInt32(destCountI.(*int32), -1)
		}
	}

	p.active.Add(-1)
	<-p.sem
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

func (p *AgentPool) ModuleCount(moduleID string) int {
	if countI, ok := p.perModule.Load(moduleID); ok {
		return int(atomic.LoadInt32(countI.(*int32)))
	}
	return 0
}

func (p *AgentPool) Stats() map[string]interface{} {
	moduleStats := make(map[string]int)
	p.perModule.Range(func(key, value interface{}) bool {
		moduleStats[key.(string)] = int(atomic.LoadInt32(value.(*int32)))
		return true
	})

	destStats := make(map[string]int)
	p.perDest.Range(func(key, value interface{}) bool {
		destStats[key.(string)] = int(atomic.LoadInt32(value.(*int32)))
		return true
	})

	return map[string]interface{}{
		"active_total":   p.ActiveCount(),
		"max_total":      p.maxTotal,
		"max_per_module": p.maxPerModule,
		"modules":        moduleStats,
		"destinations":   destStats,
	}
}
