package dag

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// Workflow is the top-level YAML structure.
// A single YAML file defines one workflow (pipeline).
type Workflow struct {
	Name        string            `yaml:"name"`
	Description string            `yaml:"description"`
	Variables   map[string]string `yaml:"variables,omitempty"`
	Nodes       []Node            `yaml:"nodes"`
}

// Node represents a single step in the workflow DAG.
// Only one of the type-specific fields should be set.
type Node struct {
	ID          string            `yaml:"id"`
	Description string            `yaml:"description,omitempty"`
	DependsOn   []string          `yaml:"depends_on,omitempty"`
	When        string            `yaml:"when,omitempty"`
	TriggerRule string            `yaml:"trigger_rule,omitempty"` // all_success (default), one_success, all_done

	// Node type -- exactly one should be set
	Prompt    *PromptNode    `yaml:"prompt,omitempty"`
	Bash      *BashNode      `yaml:"bash,omitempty"`
	Approval  *ApprovalNode  `yaml:"approval,omitempty"`
	Agent     *AgentNode     `yaml:"agent,omitempty"`
	Emit      *EmitNode      `yaml:"emit,omitempty"`
	Loop      *LoopNode      `yaml:"loop,omitempty"`

	// Retry configuration
	Retry *RetryConfig `yaml:"retry,omitempty"`

	// Model override for agent/prompt nodes
	Model string `yaml:"model,omitempty"`
}

type PromptNode struct {
	System string `yaml:"system,omitempty"`
	User   string `yaml:"user"`
}

type BashNode struct {
	Command string `yaml:"command"`
	Workdir string `yaml:"workdir,omitempty"`
}

type ApprovalNode struct {
	Message  string `yaml:"message"`
	Timeout  int    `yaml:"timeout,omitempty"` // seconds
	OnReject string `yaml:"on_reject,omitempty"` // "cancel" (default) or "skip"
}

type AgentNode struct {
	Role    string `yaml:"role"`
	Task    string `yaml:"task"`
	Timeout int    `yaml:"timeout,omitempty"`
}

type EmitNode struct {
	EventType string `yaml:"event_type"`
	Payload   string `yaml:"payload,omitempty"`
}

type LoopNode struct {
	Prompt      string `yaml:"prompt"`
	Until       string `yaml:"until"`
	MaxLoop     int    `yaml:"max_iterations,omitempty"`
	FreshCtx    bool   `yaml:"fresh_context,omitempty"`
}

type RetryConfig struct {
	MaxAttempts int    `yaml:"max_attempts"`
	DelayMs     int    `yaml:"delay_ms,omitempty"`
	OnError     string `yaml:"on_error,omitempty"` // "transient" (default) or "always"
}

// LoadWorkflow parses a YAML workflow definition and validates it.
func LoadWorkflow(data []byte) (*Workflow, error) {
	var wf Workflow
	if err := yaml.Unmarshal(data, &wf); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}

	if wf.Name == "" {
		return nil, fmt.Errorf("workflow must have a name")
	}
	if len(wf.Nodes) == 0 {
		return nil, fmt.Errorf("workflow must have at least one node")
	}

	// Validate node IDs are unique
	ids := make(map[string]bool, len(wf.Nodes))
	for _, n := range wf.Nodes {
		if n.ID == "" {
			return nil, fmt.Errorf("all nodes must have an id")
		}
		if ids[n.ID] {
			return nil, fmt.Errorf("duplicate node id: %s", n.ID)
		}
		ids[n.ID] = true
	}

	// Validate depends_on references exist
	for _, n := range wf.Nodes {
		for _, dep := range n.DependsOn {
			if !ids[dep] {
				return nil, fmt.Errorf("node %s depends_on unknown node: %s", n.ID, dep)
			}
		}
	}

	// Check for cycles
	if err := detectCycle(&wf); err != nil {
		return nil, err
	}

	return &wf, nil
}

// detectCycle uses Kahn's algorithm to detect cycles.
func detectCycle(wf *Workflow) error {
	inDegree := make(map[string]int, len(wf.Nodes))
	for _, n := range wf.Nodes {
		inDegree[n.ID] += 0 // ensure all nodes present
	}
	for _, n := range wf.Nodes {
		for _, dep := range n.DependsOn {
			inDegree[n.ID]++
			_ = dep // counted above via len
		}
	}

	queue := make([]string, 0)
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}

	visited := 0
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		visited++

		for _, n := range wf.Nodes {
			if n.ID == id {
				continue // this IS the node we just processed
			}
			for _, dep := range n.DependsOn {
				if dep == id {
					inDegree[n.ID]--
					if inDegree[n.ID] == 0 {
						queue = append(queue, n.ID)
					}
				}
			}
		}
	}

	if visited != len(wf.Nodes) {
		return fmt.Errorf("workflow contains a cycle (%d of %d nodes reachable)", visited, len(wf.Nodes))
	}
	return nil
}

// TopologicalLayers returns nodes grouped by execution layer.
// Layer 0 has no dependencies. Layer 1 depends only on layer 0. Etc.
// Nodes within the same layer can run in parallel.
func TopologicalLayers(wf *Workflow) [][]Node {
	inDegree := make(map[string]int, len(wf.Nodes))
	nodeMap := make(map[string]Node, len(wf.Nodes))
	for _, n := range wf.Nodes {
		nodeMap[n.ID] = n
		inDegree[n.ID] = len(n.DependsOn)
	}

	var layers [][]Node
	var currentLayer []Node

	for _, n := range wf.Nodes {
		if inDegree[n.ID] == 0 {
			currentLayer = append(currentLayer, n)
		}
	}

	for len(currentLayer) > 0 {
		layers = append(layers, currentLayer)
		var nextLayer []Node
		for _, n := range currentLayer {
			// Find all nodes that depend on this one
			for _, candidate := range wf.Nodes {
				for _, dep := range candidate.DependsOn {
					if dep == n.ID {
						inDegree[candidate.ID]--
						if inDegree[candidate.ID] == 0 {
							nextLayer = append(nextLayer, nodeMap[candidate.ID])
						}
					}
				}
			}
		}
		currentLayer = nextLayer
	}

	return layers
}
