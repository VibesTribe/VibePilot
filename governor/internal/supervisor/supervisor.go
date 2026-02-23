package supervisor

import (
	"context"
	"strings"

	"github.com/vibepilot/governor/pkg/types"
)

type Action string

const (
	ActionApprove Action = "approve"
	ActionReject  Action = "reject"
	ActionHuman   Action = "human"
	ActionCouncil Action = "council"
)

type Decision struct {
	Action Action
	Notes  string
	Issues []string
}

type Supervisor struct{}

func New() *Supervisor {
	return &Supervisor{}
}

func (s *Supervisor) Review(ctx context.Context, task *types.Task, packet *types.PromptPacket, outputFiles []string) Decision {
	var issues []string

	if packet == nil {
		return Decision{Action: ActionReject, Notes: "No task packet found"}
	}

	if len(packet.Deliverables) > 0 {
		for _, expected := range packet.Deliverables {
			found := false
			for _, actual := range outputFiles {
				if strings.Contains(actual, expected) || strings.Contains(expected, actual) {
					found = true
					break
				}
			}
			if !found {
				issues = append(issues, "Missing deliverable: "+expected)
			}
		}
	}

	if len(issues) > 0 {
		return Decision{
			Action: ActionReject,
			Notes:  strings.Join(issues, "; "),
			Issues: issues,
		}
	}

	if task.Type == "ui_ux" {
		return Decision{
			Action: ActionHuman,
			Notes:  "UI/UX changes require human approval",
		}
	}

	return Decision{Action: ActionApprove}
}
