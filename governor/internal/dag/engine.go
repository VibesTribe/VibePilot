package dag

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

// NodeOutput stores the result of a completed node.
type NodeOutput struct {
	Success bool
	Output  string
	Error   string
	Skipped bool
}

// NodeExecutor is the interface for executing a node type.
// Register one per node type (prompt, bash, agent, etc.)
type NodeExecutor interface {
	CanExecute(node *Node) bool
	Execute(ctx context.Context, node *Node, inputs map[string]NodeOutput) (string, error)
}

// Engine runs a Workflow DAG.
type Engine struct {
	workflow  *Workflow
	executors []NodeExecutor
	outputs   map[string]NodeOutput
	mu        sync.RWMutex
}

// NewEngine creates a DAG engine for a workflow.
func NewEngine(workflow *Workflow, executors ...NodeExecutor) *Engine {
	return &Engine{
		workflow:  workflow,
		executors: executors,
		outputs:   make(map[string]NodeOutput),
	}
}

// Run executes the workflow layer by layer.
// Within each layer, nodes run concurrently.
func (e *Engine) Run(ctx context.Context, variables map[string]string) error {
	layers := TopologicalLayers(e.workflow)

	log.Printf("[DAG] Starting workflow %q (%d nodes, %d layers)",
		e.workflow.Name, len(e.workflow.Nodes), len(layers))

	for i, layer := range layers {
		// Check context cancellation between layers
		if ctx.Err() != nil {
			return fmt.Errorf("workflow cancelled before layer %d: %w", i, ctx.Err())
		}

		log.Printf("[DAG] Layer %d: %d nodes", i, len(layer))

		if len(layer) == 1 {
			// Single node -- run directly
			out, err := e.runNode(ctx, layer[0], variables)
			if err != nil {
				return fmt.Errorf("layer %d node %s: %w", i, layer[0].ID, err)
			}
			e.setOutput(layer[0].ID, out)
		} else {
			// Multiple nodes -- run concurrently
			var wg sync.WaitGroup
			var firstErr error
			var errMu sync.Mutex

			for _, node := range layer {
				wg.Add(1)
				go func(n Node) {
					defer wg.Done()
					out, err := e.runNode(ctx, n, variables)
					if err != nil {
						errMu.Lock()
						if firstErr == nil {
							firstErr = fmt.Errorf("node %s: %w", n.ID, err)
						}
						errMu.Unlock()
						return
					}
					e.setOutput(n.ID, out)
				}(node)
			}

			wg.Wait()
			if firstErr != nil {
				return fmt.Errorf("layer %d: %w", i, firstErr)
			}
		}
	}

	log.Printf("[DAG] Workflow %q completed successfully", e.workflow.Name)
	return nil
}

// runNode executes a single node, checking conditions and finding the right executor.
func (e *Engine) runNode(ctx context.Context, node Node, variables map[string]string) (NodeOutput, error) {
	// Check when condition
	if node.When != "" {
		if !e.evaluateCondition(node.When) {
			log.Printf("[DAG] Node %s skipped (when condition: %s)", node.ID, node.When)
			return NodeOutput{Skipped: true}, nil
		}
	}

	// Collect inputs from dependencies
	inputs := make(map[string]NodeOutput)
	for _, dep := range node.DependsOn {
		inputs[dep] = e.getOutput(dep)
	}

	// Apply variable substitution to node content
	substituted := e.substituteVariables(node, variables)

	// Find the right executor
	for _, exec := range e.executors {
		if exec.CanExecute(&substituted) {
			start := time.Now()
			output, err := exec.Execute(ctx, &substituted, inputs)
			if err != nil {
				// Check retry config
				return NodeOutput{Success: false, Error: err.Error()}, err
			}
			log.Printf("[DAG] Node %s completed in %s", node.ID, time.Since(start))
			return NodeOutput{Success: true, Output: output}, nil
		}
	}

	return NodeOutput{}, fmt.Errorf("no executor found for node %s", node.ID)
}

// evaluateCondition checks a simple when expression like "$nodeid.output == 'value'"
func (e *Engine) evaluateCondition(expr string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// Simple equality check: $nodeid.output == "value"
	if strings.Contains(expr, "==") {
		parts := strings.SplitN(expr, "==", 2)
		left := strings.TrimSpace(parts[0])
		right := strings.TrimSpace(strings.Trim(strings.TrimSpace(parts[1]), "\"'"))

		// Resolve $nodeid.output references
		if strings.HasPrefix(left, "$") && strings.HasSuffix(left, ".output") {
			nodeID := strings.TrimPrefix(left, "$")
			nodeID = strings.TrimSuffix(nodeID, ".output")
			if out, ok := e.outputs[nodeID]; ok {
				return strings.TrimSpace(out.Output) == right
			}
		}
		return false
	}

	if strings.Contains(expr, "!=") {
		parts := strings.SplitN(expr, "!=", 2)
		left := strings.TrimSpace(parts[0])
		right := strings.TrimSpace(strings.Trim(strings.TrimSpace(parts[1]), "\"'"))

		if strings.HasPrefix(left, "$") && strings.HasSuffix(left, ".output") {
			nodeID := strings.TrimPrefix(left, "$")
			nodeID = strings.TrimSuffix(nodeID, ".output")
			if out, ok := e.outputs[nodeID]; ok {
				return strings.TrimSpace(out.Output) != right
			}
		}
		return false
	}

	return true
}

// substituteVariables replaces $VARIABLE references in prompt/command text.
func (e *Engine) substituteVariables(node Node, variables map[string]string) Node {
	sub := func(s string) string {
		for k, v := range variables {
			s = strings.ReplaceAll(s, "$"+k, v)
		}
		// Also substitute outputs from dependency nodes
		e.mu.RLock()
		for depID, out := range e.outputs {
			s = strings.ReplaceAll(s, "$"+depID+".output", out.Output)
		}
		e.mu.RUnlock()
		return s
	}

	result := node
	if result.Prompt != nil {
		p := *result.Prompt
		p.User = sub(p.User)
		p.System = sub(p.System)
		result.Prompt = &p
	}
	if result.Bash != nil {
		b := *result.Bash
		b.Command = sub(b.Command)
		result.Bash = &b
	}
	if result.Agent != nil {
		a := *result.Agent
		a.Task = sub(a.Task)
		result.Agent = &a
	}
	return result
}

func (e *Engine) setOutput(nodeID string, out NodeOutput) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.outputs[nodeID] = out
}

func (e *Engine) getOutput(nodeID string) NodeOutput {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.outputs[nodeID]
}

// GetOutputs returns all node outputs (for inspection after workflow completes).
func (e *Engine) GetOutputs() map[string]NodeOutput {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make(map[string]NodeOutput, len(e.outputs))
	for k, v := range e.outputs {
		result[k] = v
	}
	return result
}
