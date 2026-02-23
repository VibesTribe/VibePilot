package council

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/vibepilot/governor/internal/db"
)

type Vote string

const (
	VoteApproved       Vote = "APPROVED"
	VoteRevisionNeeded Vote = "REVISION_NEEDED"
	VoteBlocked        Vote = "BLOCKED"
)

type Lens string

const (
	LensUserAlignment     Lens = "user_alignment"
	LensTechnicalSecurity Lens = "technical_security"
	LensIdealVision       Lens = "ideal_vision"
	LensIntegration       Lens = "integration"
	LensReversibility     Lens = "reversibility"
	LensPrincipleAlign    Lens = "principle_alignment"
)

type ReviewResult struct {
	ModelID     string
	Lens        Lens
	Vote        Vote
	Confidence  float64
	Analysis    string
	Concerns    []string
	Suggestions []string
}

type PlanReviewInput struct {
	PlanID      string
	PlanType    string
	Title       string
	Description string
	Tasks       []PlanTask
	Context     map[string]interface{}
}

type PlanTask struct {
	ID           string
	Title        string
	Type         string
	Description  string
	Dependencies []string
}

type DeliberationResult struct {
	PlanID         string
	Approved       bool
	Consensus      string
	Rounds         int
	Reviews        []ReviewResult
	CommonConcerns []string
	AllSuggestions []string
}

type Council struct {
	db           *db.DB
	toolExecutor ToolExecutor
	maxRounds    int
}

type ToolExecutor interface {
	Execute(ctx context.Context, toolID, prompt string, timeoutSec int) (output string, tokensIn, tokensOut int, err error)
}

func New(database *db.DB, executor ToolExecutor) *Council {
	return &Council{
		db:           database,
		toolExecutor: executor,
		maxRounds:    4,
	}
}

func (c *Council) ReviewPlan(ctx context.Context, input *PlanReviewInput) (*DeliberationResult, error) {
	log.Printf("Council: reviewing plan %s (%s)", input.PlanID[:8], input.PlanType)

	lenses := c.determineLenses(input)

	models, err := c.db.GetAvailableModels(ctx, len(lenses))
	if err != nil {
		return nil, fmt.Errorf("get available models: %w", err)
	}
	if len(models) == 0 {
		return nil, fmt.Errorf("no models available for council review")
	}

	var allReviews []ReviewResult
	round := 1

	for round <= c.maxRounds {
		log.Printf("Council: round %d for plan %s", round, input.PlanID[:8])

		roundReviews := c.executeRound(ctx, input, lenses, models, round, allReviews)
		allReviews = append(allReviews, roundReviews...)

		consensus := c.checkConsensus(roundReviews)
		if consensus == "APPROVED" {
			log.Printf("Council: plan %s approved in round %d", input.PlanID[:8], round)
			return c.buildResult(input.PlanID, true, "unanimous", round, allReviews), nil
		}
		if consensus == "BLOCKED" {
			log.Printf("Council: plan %s blocked in round %d", input.PlanID[:8], round)
			return c.buildResult(input.PlanID, false, "blocked", round, allReviews), nil
		}

		round++
	}

	log.Printf("Council: plan %s no consensus after %d rounds", input.PlanID[:8], c.maxRounds)
	return c.buildResult(input.PlanID, false, "no_consensus", c.maxRounds, allReviews), nil
}

func (c *Council) determineLenses(input *PlanReviewInput) []Lens {
	switch input.PlanType {
	case "new_project":
		return []Lens{LensUserAlignment, LensIdealVision, LensTechnicalSecurity}
	case "system_improvement":
		return []Lens{LensIntegration, LensReversibility, LensPrincipleAlign}
	case "feature":
		return []Lens{LensUserAlignment, LensIntegration, LensTechnicalSecurity}
	default:
		return []Lens{LensUserAlignment, LensIdealVision, LensTechnicalSecurity}
	}
}

func (c *Council) executeRound(ctx context.Context, input *PlanReviewInput, lenses []Lens, models []db.Runner, round int, previousReviews []ReviewResult) []ReviewResult {
	var reviews []ReviewResult
	var wg sync.WaitGroup
	var mu sync.Mutex

	prompt := c.buildReviewPrompt(input, round, previousReviews)

	for i, lens := range lenses {
		wg.Add(1)
		go func(idx int, l Lens) {
			defer wg.Done()

			modelIdx := idx % len(models)
			model := models[modelIdx]

			lensPrompt := c.addLensContext(prompt, l, input)
			output, _, _, err := c.toolExecutor.Execute(ctx, model.ToolID, lensPrompt, 120)
			if err != nil {
				log.Printf("Council: model %s failed for lens %s: %v", model.ModelID, l, err)
				return
			}

			review := c.parseReviewOutput(output, model.ModelID, l)
			mu.Lock()
			reviews = append(reviews, review)
			mu.Unlock()

			_, err = c.db.SubmitCouncilReview(ctx, &db.CouncilReviewInput{
				PlanID:      input.PlanID,
				Round:       round,
				ModelID:     model.ModelID,
				Lens:        string(l),
				Vote:        string(review.Vote),
				Confidence:  review.Confidence,
				Approach:    review.Analysis,
				Concerns:    review.Concerns,
				Suggestions: review.Suggestions,
			})
			if err != nil {
				log.Printf("Council: failed to store review: %v", err)
			}
		}(i, lens)
	}

	wg.Wait()
	return reviews
}

func (c *Council) buildReviewPrompt(input *PlanReviewInput, round int, previousReviews []ReviewResult) string {
	var sb strings.Builder

	sb.WriteString("COUNCIL PLAN REVIEW\n\n")
	sb.WriteString(fmt.Sprintf("Plan Type: %s\n", input.PlanType))
	sb.WriteString(fmt.Sprintf("Title: %s\n", input.Title))
	sb.WriteString(fmt.Sprintf("Description: %s\n\n", input.Description))

	if len(input.Tasks) > 0 {
		sb.WriteString("TASKS:\n")
		for i, task := range input.Tasks {
			sb.WriteString(fmt.Sprintf("%d. %s (%s)\n", i+1, task.Title, task.Type))
			if task.Description != "" {
				sb.WriteString(fmt.Sprintf("   %s\n", task.Description))
			}
			if len(task.Dependencies) > 0 {
				sb.WriteString(fmt.Sprintf("   Dependencies: %s\n", strings.Join(task.Dependencies, ", ")))
			}
		}
	}

	if round > 1 && len(previousReviews) > 0 {
		sb.WriteString("\nPREVIOUS ROUND FEEDBACK:\n")
		for _, rev := range previousReviews {
			sb.WriteString(fmt.Sprintf("- %s (%s): %s\n", rev.ModelID, rev.Lens, rev.Vote))
			if len(rev.Concerns) > 0 {
				sb.WriteString(fmt.Sprintf("  Concerns: %s\n", strings.Join(rev.Concerns, ", ")))
			}
			if len(rev.Suggestions) > 0 {
				sb.WriteString(fmt.Sprintf("  Suggestions: %s\n", strings.Join(rev.Suggestions, ", ")))
			}
		}
	}

	sb.WriteString("\nREVIEW INSTRUCTIONS:\n")
	sb.WriteString("Analyze this plan from your assigned lens perspective.\n")
	sb.WriteString("Consider alignment with user intent, technical soundness, integration impact.\n")
	sb.WriteString("Respond in JSON format:\n")
	sb.WriteString("{\n")
	sb.WriteString("  \"vote\": \"APPROVED\" | \"REVISION_NEEDED\" | \"BLOCKED\",\n")
	sb.WriteString("  \"confidence\": 0.0-1.0,\n")
	sb.WriteString("  \"analysis\": \"Brief assessment\",\n")
	sb.WriteString("  \"concerns\": [\"list of concerns\"],\n")
	sb.WriteString("  \"suggestions\": [\"list of suggestions\"]\n")
	sb.WriteString("}\n")

	return sb.String()
}

func (c *Council) addLensContext(prompt string, lens Lens, input *PlanReviewInput) string {
	var lensContext string

	switch lens {
	case LensUserAlignment:
		lensContext = "\n\nYOUR LENS: USER ALIGNMENT\nConsider: Does this match the user's stated intent? Will it deliver what they asked for?"

	case LensIdealVision:
		lensContext = "\n\nYOUR LENS: IDEAL VISION\nConsider: Is this the best approach? Could it be simpler? Does it solve the right problem?"

	case LensTechnicalSecurity:
		lensContext = "\n\nYOUR LENS: TECHNICAL & SECURITY\nConsider: Is the implementation sound? Any security concerns? Performance implications?"

	case LensIntegration:
		lensContext = "\n\nYOUR LENS: INTEGRATION\nConsider: Does this fit existing system? Breaking changes? Dependencies managed?"

	case LensReversibility:
		lensContext = "\n\nYOUR LENS: REVERSIBILITY\nConsider: Can this be undone? Rollback path? Migration safe?"

	case LensPrincipleAlign:
		lensContext = "\n\nYOUR LENS: PRINCIPLE ALIGNMENT\nConsider: Does this follow VibePilot principles? Zero lock-in? Modular? Exit ready?"
	}

	return prompt + lensContext
}

func (c *Council) parseReviewOutput(output string, modelID string, lens Lens) ReviewResult {
	review := ReviewResult{
		ModelID: modelID,
		Lens:    lens,
		Vote:    VoteRevisionNeeded,
	}

	output = strings.TrimSpace(output)
	idx := strings.Index(output, "{")
	if idx == -1 {
		return review
	}
	jsonStr := output[idx:]
	lastIdx := strings.LastIndex(jsonStr, "}")
	if lastIdx == -1 {
		return review
	}
	jsonStr = jsonStr[:lastIdx+1]

	var parsed struct {
		Vote        string   `json:"vote"`
		Confidence  float64  `json:"confidence"`
		Analysis    string   `json:"analysis"`
		Concerns    []string `json:"concerns"`
		Suggestions []string `json:"suggestions"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		log.Printf("Council: failed to parse review output: %v", err)
		return review
	}

	review.Vote = Vote(strings.ToUpper(parsed.Vote))
	review.Confidence = parsed.Confidence
	review.Analysis = parsed.Analysis
	review.Concerns = parsed.Concerns
	review.Suggestions = parsed.Suggestions

	return review
}

func (c *Council) checkConsensus(reviews []ReviewResult) string {
	if len(reviews) == 0 {
		return "NO_REVIEWS"
	}

	approved := 0
	blocked := 0

	for _, r := range reviews {
		switch r.Vote {
		case VoteApproved:
			approved++
		case VoteBlocked:
			blocked++
		}
	}

	if blocked > 0 {
		return "BLOCKED"
	}
	if approved == len(reviews) {
		return "APPROVED"
	}
	return "NO_CONSENSUS"
}

func (c *Council) buildResult(planID string, approved bool, consensus string, rounds int, reviews []ReviewResult) *DeliberationResult {
	var commonConcerns []string
	var allSuggestions []string
	concernCounts := make(map[string]int)

	for _, r := range reviews {
		for _, concern := range r.Concerns {
			concernCounts[concern]++
		}
		allSuggestions = append(allSuggestions, r.Suggestions...)
	}

	for concern, count := range concernCounts {
		if count > 1 {
			commonConcerns = append(commonConcerns, concern)
		}
	}

	return &DeliberationResult{
		PlanID:         planID,
		Approved:       approved,
		Consensus:      consensus,
		Rounds:         rounds,
		Reviews:        reviews,
		CommonConcerns: commonConcerns,
		AllSuggestions: allSuggestions,
	}
}
