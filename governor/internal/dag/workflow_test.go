package dag

import (
	"os"
	"testing"
)

func TestLoadCodePipeline(t *testing.T) {
	data, err := os.ReadFile("../../config/pipelines/code-pipeline.yaml")
	if err != nil {
		t.Fatalf("read pipeline: %v", err)
	}

	wf, err := LoadWorkflow(data)
	if err != nil {
		t.Fatalf("parse pipeline: %v", err)
	}

	if wf.Name != "code-pipeline" {
		t.Errorf("name = %q, want %q", wf.Name, "code-pipeline")
	}
	if len(wf.Nodes) == 0 {
		t.Error("no nodes loaded")
	}

	// Check that layers make sense
	layers := TopologicalLayers(wf)
	if len(layers) == 0 {
		t.Error("no layers produced")
	}

	t.Logf("Workflow: %s (%d nodes, %d layers)", wf.Name, len(wf.Nodes), len(layers))
	for i, layer := range layers {
		var ids []string
		for _, n := range layer {
			ids = append(ids, n.ID)
		}
		t.Logf("  Layer %d: %v", i, ids)
	}
}

func TestDetectCycle(t *testing.T) {
	yaml := `
name: cycle-test
nodes:
  - id: a
    depends_on: [b]
    prompt: {user: "a"}
  - id: b
    depends_on: [a]
    prompt: {user: "b"}
`
	_, err := LoadWorkflow([]byte(yaml))
	if err == nil {
		t.Error("expected cycle detection error")
	}
	t.Logf("Got expected error: %v", err)
}
