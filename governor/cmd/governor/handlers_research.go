package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/vibepilot/governor/internal/db"
	"github.com/vibepilot/governor/internal/runtime"
)

type ResearchHandler struct {
	database      db.Database
	factory       *runtime.SessionFactory
	pool          *runtime.AgentPool
	connRouter    *runtime.Router
	cfg           *runtime.Config
	usageTracker  *runtime.UsageTracker
	actionApplier *runtime.ResearchActionApplier
}

// councilMemberResult holds a single council member's per-item votes.
type councilMemberResult struct {
	items map[int]map[string]any // item sort_order -> {vote, reasoning, concerns}
	err   error
}

// reportItemResult holds the aggregated result for a single report item.
type reportItemResult struct {
	sortOrder      int
	title          string
	recommendation string
	reasoning      string
	concerns       []string
	councilVotes   []map[string]any
}

func NewResearchHandler(
	database db.Database,
	factory *runtime.SessionFactory,
	pool *runtime.AgentPool,
	connRouter *runtime.Router,
	cfg *runtime.Config,
	usageTracker *runtime.UsageTracker,
	actionApplier *runtime.ResearchActionApplier,
) *ResearchHandler {
	return &ResearchHandler{
		database:      database,
		factory:       factory,
		pool:          pool,
		connRouter:    connRouter,
		cfg:           cfg,
		usageTracker:  usageTracker,
		actionApplier: actionApplier,
	}
}

func (h *ResearchHandler) Register(router *runtime.EventRouter) {
	router.On(runtime.EventResearchReady, h.handleResearchReady)
	router.On(runtime.EventResearchCouncil, h.handleResearchCouncil)
	// New event: report is ready for council (all items collected)
	router.On(runtime.EventReportCouncil, h.handleReportCouncilReview)
	// Human approved research -> route to consultant for PRD creation
	router.On(runtime.EventResearchApproved, h.handleResearchApproved)
}

// handleResearchReady fires for each individual research_suggestion.
// It creates or finds a daily report and adds the suggestion as an item.
// Once all expected items are added, it routes the report to council.
func (h *ResearchHandler) handleResearchReady(event runtime.Event) {
	ctx := context.Background()

	suggestion, err := fetchRecord(ctx, h.database, event)
	if err != nil {
		log.Printf("[ResearchReady] Failed to get suggestion record: %v", err)
		return
	}

	suggestionID := getString(suggestion, "id")
	suggestionType := getString(suggestion, "type")
	complexity := getString(suggestion, "complexity")
	title := getString(suggestion, "title")
	summary := getString(suggestion, "summary")
	findingsPath := getString(suggestion, "findings_path")

	if suggestionID == "" {
		return
	}

	processingBy := fmt.Sprintf("research_ready:%d", time.Now().UnixNano())
	claimed, err := h.database.RPC(ctx, "set_processing", map[string]any{
		"p_table":         "research_suggestions",
		"p_id":            suggestionID,
		"p_processing_by": processingBy,
	})
	if err != nil || !parseBool(claimed) {
		log.Printf("[ResearchReady] Suggestion %s already being processed", truncateID(suggestionID))
		return
	}

	defer h.database.RPC(ctx, "clear_processing", map[string]any{
		"p_table": "research_suggestions",
		"p_id":    suggestionID,
	})

	// Mark suggestion as processing
	_, _ = h.database.RPC(ctx, "update_research_suggestion_status", map[string]any{
		"p_id":     suggestionID,
		"p_status": "council_review",
		"p_review_notes": map[string]any{
			"source":              "research",
			"type":                suggestionType,
			"original_complexity": complexity,
		},
	})

	// Find or create today's daily report
	details, _ := suggestion["details"].(map[string]any)
	reportID := h.findOrCreateDailyReport(ctx, findingsPath, details)

	// Add this suggestion as an item in the report
	h.addReportItem(ctx, reportID, suggestionID, title, summary, suggestionType, details)

	log.Printf("[ResearchReady] Added %s to report %s", truncateID(suggestionID), truncateID(reportID))

	// Check if report has enough items to send to council
	// For daily scans: send when we have items (council will be triggered by the report status change)
	// The pg_notify trigger on research_reports handles routing to council
	h.checkAndRouteReport(ctx, reportID)
}

// findOrCreateDailyReport gets today's daily_scan report or creates one.
func (h *ResearchHandler) findOrCreateDailyReport(ctx context.Context, findingsPath string, details map[string]any) string {
	today := time.Now().Format("2006-01-02")
	reportTitle := fmt.Sprintf("Daily Research Scan - %s", today)

	// Check for existing report today
	data, err := h.database.Query(ctx, "research_reports", map[string]any{
		"report_type": "daily_scan",
		"limit":       1,
	})
	if err == nil {
		var reports []map[string]any
		if json.Unmarshal(data, &reports) == nil && len(reports) > 0 {
			if id, ok := reports[0]["id"].(string); ok && id != "" {
				return id
			}
		}
	}

	// Create new report
	result, err := h.database.Insert(ctx, "research_reports", map[string]any{
		"title":         reportTitle,
		"report_type":   "daily_scan",
		"source":        "researcher",
		"status":        "council_review",
		"findings_path": findingsPath,
	})
	if err != nil {
		log.Printf("[ResearchReady] Failed to create report: %v", err)
		return ""
	}

	var created []map[string]any
	if json.Unmarshal(result, &created) == nil && len(created) > 0 {
		if id, ok := created[0]["id"].(string); ok {
			return id
		}
	}
	return ""
}

// addReportItem adds a suggestion as an item in a research report.
func (h *ResearchHandler) addReportItem(ctx context.Context, reportID, suggestionID, title, summary, findingType string, details map[string]any) {
	if reportID == "" {
		return
	}

	// Check if already added
	existing, _ := h.database.Query(ctx, "research_report_items", map[string]any{
		"report_id":     reportID,
		"suggestion_id": suggestionID,
	})
	if existing != nil {
		var items []map[string]any
		if json.Unmarshal(existing, &items) == nil && len(items) > 0 {
			return // already added
		}
	}

	// Get current item count for sort order
	count := 0
	countData, _ := h.database.Query(ctx, "research_report_items", map[string]any{
		"report_id": reportID,
		"select":    "count",
	})
	if countData != nil {
		var items []map[string]any
		if json.Unmarshal(countData, &items) == nil {
			count = len(items)
		}
	}

	detailsJSON := "{}"
	if details != nil {
		if b, err := json.Marshal(details); err == nil {
			detailsJSON = string(b)
		}
	}

	// Extract comparison fields from researcher output
	currentState := extractComparisonField(details, "current_state")
	newThing := extractComparisonField(details, "new_thing")
	improvement := extractComparisonField(details, "improvement")

	_, err := h.database.Insert(ctx, "research_report_items", map[string]any{
		"report_id":     reportID,
		"suggestion_id": suggestionID,
		"title":         title,
		"finding_type":  findingType,
		"summary":       summary,
		"details":       detailsJSON,
		"current_state": currentState,
		"new_thing":     newThing,
		"improvement":   improvement,
		"sort_order":    count,
	})
	if err != nil {
		log.Printf("[ResearchReady] Failed to add report item: %v", err)
	}
}

// checkAndRouteReport checks if a report should be sent to council.
// For now, routes immediately since items arrive one at a time.
// The report stays in council_review status and council processes all items together.
func (h *ResearchHandler) checkAndRouteReport(ctx context.Context, reportID string) {
	if reportID == "" {
		return
	}

	// Count items in this report
	data, err := h.database.Query(ctx, "research_report_items", map[string]any{
		"report_id": reportID,
	})
	if err != nil {
		return
	}
	var items []map[string]any
	if json.Unmarshal(data, &items) != nil {
		return
	}

	log.Printf("[ResearchReady] Report %s has %d items", truncateID(reportID), len(items))

	// For now, we don't auto-trigger council here.
	// The daily research cron will call /api/research-reports/{id}/trigger-council
	// when all items are collected. Or we trigger immediately for single items.
	// The report is already in council_review status, so the pg_notify trigger
	// on research_reports fires the research_report_council_review event.
}

// handleReportCouncilReview runs the council on all items in a report.
// Council analyzes each item and provides per-item recommendations.
func (h *ResearchHandler) handleReportCouncilReview(event runtime.Event) {
	ctx := context.Background()

	// Extract report_id from event ID or Record
	reportID := event.ID
	if reportID == "" {
		// Try parsing from record
		var record map[string]any
		if json.Unmarshal(event.Record, &record) == nil {
			reportID = getString(record, "id")
		}
	}
	if reportID == "" {
		log.Printf("[ReportCouncil] No report_id in event")
		return
	}

	// Fetch report with items
	reportData, err := h.database.RPC(ctx, "get_report_for_review", map[string]any{
		"p_report_id": reportID,
	})
	if err != nil {
		log.Printf("[ReportCouncil] Failed to fetch report %s: %v", truncateID(reportID), err)
		return
	}

	// pgx RPC wraps jsonb results as [{"fn_name":{...}}] -- unwrap.
	reportData = unwrapRPCResult(reportData, "get_report_for_review")

	log.Printf("[ReportCouncil] After unwrap, data length=%d, first200=%s", len(reportData), string(reportData[:min(len(reportData), 200)]))

	var report map[string]any
	if json.Unmarshal(reportData, &report) != nil {
		log.Printf("[ReportCouncil] Failed to parse report data (%d bytes): %s", len(reportData), string(reportData[:min(len(reportData), 300)]))
		return
	}

	itemsRaw, _ := report["items"].([]any)
	if len(itemsRaw) == 0 {
		log.Printf("[ReportCouncil] Report %s has no items", truncateID(reportID))
		return
	}

	log.Printf("[ReportCouncil] Starting sequential council review for report %s (%d items)", truncateID(reportID), len(itemsRaw))

	// Load the original research report content so council members can read it
	reportContent := h.loadReportContent(ctx, report)

	// Load current system context so council can evaluate findings against reality
	var systemContext string
	if reportItems, ok := itemsRaw[0].(map[string]any); ok {
		itemType, _ := reportItems["finding_type"].(string)
		if itemType == "" {
			itemType = "architecture"
		}
		systemContext = h.loadKBContext(ctx, itemType)
	}

	memberCount := h.cfg.GetCouncilMemberCount()
	lenses := h.cfg.GetCouncilLenses()
	if len(lenses) == 0 {
		lenses = []string{"user_alignment", "architecture", "feasibility"}
	}
	if memberCount <= 0 {
		memberCount = 3
	}

	// SEQUENTIAL council: run member 1, wait, then member 2, wait, then member 3.
	// No goroutines, no mutex, no races. Each member sees full report + prior member comments.
	results := make([]councilMemberResult, memberCount)
	var failedModels []string

	for i := 0; i < memberCount; i++ {
		lens := lenses[i%len(lenses)]
		log.Printf("[ReportCouncil] Running member %d/%d (lens: %s)", i+1, memberCount, lens)

		memberRouting, routeErr := h.connRouter.SelectRouting(ctx, runtime.RoutingRequest{
			Role:          "council",
			TaskType:      "research_council",
			RoutingFlag:   "internal",
			ExcludeModels: failedModels,
		})
		if routeErr != nil || memberRouting == nil {
			log.Printf("[ReportCouncil] No routing for member %d, skipping", i+1)
			continue
		}

		session, err := h.factory.CreateWithConnector(ctx, "research_council", lens, memberRouting.ConnectorID)
		if err != nil {
			log.Printf("[ReportCouncil] Failed to create session for member %d: %v", i+1, err)
			failedModels = append(failedModels, memberRouting.ModelID)
			continue
		}

		// Build context: full report, original content, system context, and prior member votes for context
		contextData := map[string]any{
			"report":          report,
			"items":           itemsRaw,
			"original_report": reportContent,
			"system_context":  systemContext,
			"lens":            lens,
			"member_number":   i + 1,
			"total_members":   memberCount,
			"review_type":     "research_report",
		}
		// Feed prior members' votes so later members can build on them
		if i > 0 {
			var priorVotes []map[string]any
			for pi, pr := range results[:i] {
				if pr.err != nil {
					priorVotes = append(priorVotes, map[string]any{"member": pi + 1, "error": pr.err.Error()})
					continue
				}
				if pr.items != nil {
					priorVotes = append(priorVotes, map[string]any{"member": pi + 1, "votes": pr.items})
				}
			}
			if len(priorVotes) > 0 {
				contextData["prior_member_votes"] = priorVotes
			}
		}

		memberStart := time.Now()
		result, runErr := session.Run(ctx, contextData)
		memberDuration := time.Since(memberStart).Seconds()

		if runErr != nil {
			log.Printf("[ReportCouncil] Member %d failed: %v", i+1, runErr)
			failedModels = append(failedModels, memberRouting.ModelID)
			if h.usageTracker != nil {
				h.usageTracker.RecordCompletion(ctx, memberRouting.ModelID, "research_council", memberDuration, false)
			}
			results[i] = councilMemberResult{err: runErr}
			continue
		}

		// Parse per-item votes from council output
		itemVotes := runtime.ParseReportCouncilVotes(result.Output)
		results[i] = councilMemberResult{items: itemVotes}

		if h.usageTracker != nil {
			h.usageTracker.RecordCompletion(ctx, memberRouting.ModelID, "research_council", memberDuration, true)
		}

		log.Printf("[ReportCouncil] Member %d/%d (%s) completed review", i+1, memberCount, lens)
	}

	// Aggregate per-item recommendations
	h.aggregateAndSaveCouncilResults(ctx, reportID, report, results, memberCount)

	log.Printf("[ReportCouncil] Report %s council review complete (%d/%d members succeeded)",
		truncateID(reportID), memberCount-len(failedModels), memberCount)
}

// loadReportContent reads the original research findings file so council can review it.
func (h *ResearchHandler) loadReportContent(ctx context.Context, report map[string]any) string {
	findingsPath := getString(report, "findings_path")
	if findingsPath == "" {
		return ""
	}
	// Strip knowledgebase/ prefix if present to avoid doubling
	kbBase := "/home/vibes/knowledgebase"
	cleanPath := strings.TrimPrefix(findingsPath, "knowledgebase/")
	cleanPath = strings.TrimPrefix(cleanPath, "/")
	fullPath := kbBase + "/" + cleanPath
	data, err := os.ReadFile(fullPath)
	if err != nil {
		log.Printf("[ReportCouncil] Could not read original report at %s: %v", fullPath, err)
		return ""
	}
	content := string(data)
	// Cap at 8000 chars to avoid blowing up context
	if len(content) > 8000 {
		content = content[:8000] + "\n...[truncated]"
	}
	return content
}

// aggregateAndSaveCouncilResults merges council votes into per-item recommendations
// and writes the KB decision doc.
func (h *ResearchHandler) aggregateAndSaveCouncilResults(
	ctx context.Context,
	reportID string,
	report map[string]any,
	results []councilMemberResult,
	memberCount int,
) {
	// Collect all items from the report
	itemsRaw, _ := report["items"].([]any)

	type itemAgg struct {
		approves   int
		rejects    int
		watches    int
		reasonings []string
		concerns   []string
	}

	aggregates := make(map[int]*itemAgg)
	for _, itemRaw := range itemsRaw {
		item, _ := itemRaw.(map[string]any)
		sortOrder := 0
		if so, ok := item["sort_order"].(float64); ok {
			sortOrder = int(so)
		}
		aggregates[sortOrder] = &itemAgg{}
	}

	// Tally votes from each council member
	for _, res := range results {
		if res.err != nil || res.items == nil {
			continue
		}
		for sortIdx, vote := range res.items {
			agg, ok := aggregates[sortIdx]
			if !ok {
				continue
			}
			v := getString(vote, "vote")
			switch strings.ToLower(v) {
			case "approve", "approved":
				agg.approves++
			case "reject", "rejected":
				agg.rejects++
			case "watch":
				agg.watches++
			}
			if r := getString(vote, "reasoning"); r != "" {
				agg.reasonings = append(agg.reasonings, r)
			}
			if c, ok := vote["concerns"].([]any); ok {
				for _, ci := range c {
					if s, ok := ci.(string); ok {
						agg.concerns = append(agg.concerns, s)
					}
				}
			}
		}
	}

	// Determine recommendation per item
	var itemResults []reportItemResult

	for _, itemRaw := range itemsRaw {
		item, _ := itemRaw.(map[string]any)
		sortOrder := 0
		if so, ok := item["sort_order"].(float64); ok {
			sortOrder = int(so)
		}

		agg := aggregates[sortOrder]
		title := getString(item, "title")

		var recommendation string
		if agg.approves > memberCount/2 {
			recommendation = "approve"
		} else if agg.rejects > memberCount/2 {
			recommendation = "reject"
		} else {
			recommendation = "watch"
		}

		reasoning := "Council did not produce detailed reasoning."
		if len(agg.reasonings) > 0 {
			// Combine all member reasoning into a readable summary
			if len(agg.reasonings) == 1 {
				reasoning = agg.reasonings[0]
			} else {
				var parts []string
				for mi, r := range agg.reasonings {
					parts = append(parts, fmt.Sprintf("Member %d: %s", mi+1, r))
				}
				reasoning = strings.Join(parts, " | ")
			}
		} else if agg.approves > 0 || agg.rejects > 0 || agg.watches > 0 {
			reasoning = fmt.Sprintf("Council voted: %d approve, %d watch, %d reject (no detailed reasoning provided).",
				agg.approves, agg.watches, agg.rejects)
		}

		// Collect per-member votes for this item
		var councilVotes []map[string]any
		for mi, res := range results {
			if res.err != nil || res.items == nil {
				continue
			}
			if vote, ok := res.items[sortOrder]; ok {
				councilVotes = append(councilVotes, map[string]any{
					"member": mi + 1,
					"vote":   getString(vote, "vote"),
					"reasoning": getString(vote, "reasoning"),
				})
			}
		}

		itemResults = append(itemResults, reportItemResult{
			sortOrder:      sortOrder,
			title:          title,
			recommendation: recommendation,
			reasoning:      reasoning,
			concerns:       agg.concerns,
			councilVotes:   councilVotes,
		})

		// Update the report item in DB
		concernsJSON, _ := json.Marshal(agg.concerns)
		_, _ = h.database.RPC(ctx, "update_report_item_council", map[string]any{
			"p_report_id":            reportID,
			"p_sort_order":           sortOrder,
			"p_council_recommendation": recommendation,
			"p_council_reasoning":     reasoning,
			"p_council_concerns":      string(concernsJSON),
		})
	}

	// Write KB decision doc
	findingsPath := getString(report, "findings_path")
	reportTitle := getString(report, "title")
	kbDecisionPath := h.writeReportDecisionDoc(ctx, reportTitle, reportID, findingsPath, itemResults)

	// Update report status to pending_human
	_, _ = h.database.RPC(ctx, "update_report_status", map[string]any{
		"p_id":               reportID,
		"p_status":           "pending_human",
		"p_decision_doc_path": kbDecisionPath,
	})

	// Insert a review_items entry so it shows in Review Hub
	// Include item summaries with comparison data in the payload
	var reviewPayload []map[string]any
	for _, ir := range itemResults {
		reviewPayload = append(reviewPayload, map[string]any{
			"title":          ir.title,
			"recommendation": ir.recommendation,
			"reasoning":      ir.reasoning,
		})
	}
	reviewSummary := fmt.Sprintf("Council reviewed %d items. Decision doc ready.", len(itemResults))
	h.insertReviewItem(ctx, "research", reportID, reportTitle,
		reviewSummary, "high", map[string]any{
			"items":         reviewPayload,
			"decision_doc":  kbDecisionPath,
			"report_id":     reportID,
		})

	log.Printf("[ReportCouncil] Report %s -> pending_human (%d items reviewed)", truncateID(reportID), len(itemResults))
}

// handleResearchCouncil handles the old per-suggestion council event.
// Kept for backward compatibility but routes to the new report-based flow.
func (h *ResearchHandler) handleResearchCouncil(event runtime.Event) {
	ctx := context.Background()

	suggestion, err := fetchRecord(ctx, h.database, event)
	if err != nil {
		log.Printf("[ResearchCouncil] Failed to get suggestion record: %v", err)
		return
	}

	suggestionID := getString(suggestion, "id")
	if suggestionID == "" {
		return
	}

	// Check if this suggestion is already part of a report
	data, _ := h.database.Query(ctx, "research_report_items", map[string]any{
		"suggestion_id": suggestionID,
		"limit":         1,
	})
	var existingItems []map[string]any
	if json.Unmarshal(data, &existingItems) == nil && len(existingItems) > 0 {
		// Already handled by report-based flow
		log.Printf("[ResearchCouncil] Suggestion %s handled by report flow, skipping", truncateID(suggestionID))
		return
	}

	// Legacy: create a single-item report for orphaned suggestions
	reportTitle := getString(suggestion, "title")
	findingsPath := getString(suggestion, "findings_path")

	result, err := h.database.Insert(ctx, "research_reports", map[string]any{
		"title":         reportTitle,
		"report_type":   "manual",
		"source":        "legacy",
		"status":        "council_review",
		"findings_path": findingsPath,
	})
	if err != nil {
		log.Printf("[ResearchCouncil] Failed to create legacy report: %v", err)
		return
	}

	var reports []map[string]any
	if json.Unmarshal(result, &reports) != nil || len(reports) == 0 {
		return
	}
	reportID, _ := reports[0]["id"].(string)

	// Add the suggestion as a report item
	h.addReportItem(ctx, reportID, suggestionID, reportTitle,
		getString(suggestion, "summary"),
		getString(suggestion, "type"),
		nil)

	// Trigger the report-based council review
	h.handleReportCouncilReview(runtime.Event{
		Type: runtime.EventReportCouncil,
		ID:   reportID,
	})
}

// handleResearchApproved fires when a human approves a research suggestion in Review Hub.
// It routes the approved item to the consultant agent to create a PRD, which then
// enters the normal planning pipeline (consultant -> PRD -> planner -> tasks).
func (h *ResearchHandler) handleResearchApproved(event runtime.Event) {
	ctx := context.Background()

	suggestion, err := fetchRecord(ctx, h.database, event)
	if err != nil {
		log.Printf("[ResearchApproved] Failed to get suggestion record: %v", err)
		return
	}

	suggestionID := getString(suggestion, "id")
	title := getString(suggestion, "title")
	summary := getString(suggestion, "summary")
	findingsPath := getString(suggestion, "findings_path")
	suggestionType := getString(suggestion, "type")
	details, _ := suggestion["details"].(map[string]any)

	if suggestionID == "" {
		return
	}

	// Prevent double-processing
	processingBy := fmt.Sprintf("consultant:%d", time.Now().UnixNano())
	claimed, err := h.database.RPC(ctx, "set_processing", map[string]any{
		"p_table":         "research_suggestions",
		"p_id":            suggestionID,
		"p_processing_by": processingBy,
	})
	if err != nil || !parseBool(claimed) {
		log.Printf("[ResearchApproved] Suggestion %s already being processed", truncateID(suggestionID))
		return
	}

	defer h.database.RPC(ctx, "clear_processing", map[string]any{
		"p_table": "research_suggestions",
		"p_id":    suggestionID,
	})

	// Load the original research findings
	researchContent := h.loadReportContent(ctx, map[string]any{"findings_path": findingsPath})

	// Load current system context so the consultant knows what we have now
	kbContext := ""
	if h.actionApplier != nil {
		// Use the context builder if available
		kbContext = h.loadKBContext(ctx, suggestionType)
	}

	// Build consultant context
	consultantContext := map[string]any{
		"action":           "generate_prd",
		"source":           "research_approved",
		"suggestion_id":    suggestionID,
		"title":            title,
		"summary":          summary,
		"type":             suggestionType,
		"details":          details,
		"research_content": researchContent,
		"current_system":   kbContext,
	}

	// Route to consultant agent
	routingResult, routeErr := h.connRouter.SelectRouting(ctx, runtime.RoutingRequest{
		Role:        "consultant",
		TaskType:    "research_prd",
		RoutingFlag: "internal",
	})
	if routeErr != nil || routingResult == nil {
		log.Printf("[ResearchApproved] No routing for consultant, trying planner role: %v", routeErr)
		// Fallback to any available internal model
		routingResult, routeErr = h.connRouter.SelectRouting(ctx, runtime.RoutingRequest{
			Role:        "analyst",
			TaskType:    "research_prd",
			RoutingFlag: "internal",
		})
		if routeErr != nil || routingResult == nil {
			log.Printf("[ResearchApproved] No available model for consultant PRD generation")
			return
		}
	}

	session, err := h.factory.CreateWithConnector(ctx, "consultant", "research_prd", routingResult.ConnectorID)
	if err != nil {
		log.Printf("[ResearchApproved] Failed to create consultant session: %v", err)
		return
	}

	start := time.Now()
	result, runErr := session.Run(ctx, consultantContext)
	duration := time.Since(start).Seconds()

	if runErr != nil {
		log.Printf("[ResearchApproved] Consultant session failed: %v", runErr)
		if h.usageTracker != nil {
			h.usageTracker.RecordCompletion(ctx, routingResult.ModelID, "consultant", duration, false)
		}
		return
	}

	if h.usageTracker != nil {
		h.usageTracker.RecordCompletion(ctx, routingResult.ModelID, "consultant", duration, true)
	}

	// Extract PRD from result
	prdContent := result.Output
	if prdContent == "" {
		log.Printf("[ResearchApproved] Consultant returned empty output for %s", truncateID(suggestionID))
		return
	}

	// Write PRD to disk
	slug := slugFromTitle(title)
	prdPath := fmt.Sprintf("docs/prd/%s.md", slug)
	repoBase := "/home/vibes/vibepilot"
	fullPath := repoBase + "/" + prdPath
	if err := os.MkdirAll(fullPath[:strings.LastIndex(fullPath, "/")], 0755); err != nil {
		log.Printf("[ResearchApproved] Failed to create PRD dir: %v", err)
		return
	}
	if err := os.WriteFile(fullPath, []byte(prdContent), 0644); err != nil {
		log.Printf("[ResearchApproved] Failed to write PRD: %v", err)
		return
	}

	// Commit and push
	commitMsg := fmt.Sprintf("prd: %s (from approved research)", title)
	cmd := fmt.Sprintf("cd %s && git add %s && git commit -m %s && git push",
		repoBase, prdPath, shellQuote(commitMsg))
	if output, err := execCommand("bash", "-c", cmd); err != nil {
		log.Printf("[ResearchApproved] Git push failed: %v, output: %s", err, string(output))
		// Still continue - the PRD is on disk and can be committed later
	} else {
		log.Printf("[ResearchApproved] PRD committed and pushed: %s", prdPath)
	}

	// Create plan via RPC (triggers the planner pipeline)
	_, _ = h.database.RPC(ctx, "create_plan", map[string]any{
		"p_project_id": nil,
		"p_prd_path":   prdPath,
		"p_plan_path":  nil,
	})

	// Update suggestion status to 'prd_created'
	_, _ = h.database.RPC(ctx, "update_research_suggestion_status", map[string]any{
		"p_id":     suggestionID,
		"p_status": "prd_created",
		"p_review_notes": map[string]any{
			"approved_by": "human",
			"prd_path":    prdPath,
			"triggered_at": time.Now().Format(time.RFC3339),
		},
	})

	log.Printf("[ResearchApproved] PRD created for '%s' at %s, plan triggered", title, prdPath)
}

// slugFromTitle creates a URL/filename-safe slug from a title.
func slugFromTitle(title string) string {
	slug := strings.ToLower(title)
	// Replace spaces and special chars with hyphens
	slug = strings.ReplaceAll(slug, " ", "-")
	// Remove non-alphanumeric chars (keep hyphens)
	var cleaned strings.Builder
	for _, r := range slug {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			cleaned.WriteRune(r)
		}
	}
	slug = cleaned.String()
	// Collapse multiple hyphens
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}
	slug = strings.Trim(slug, "-")
	if len(slug) > 60 {
		slug = slug[:60]
	}
	if slug == "" {
		slug = fmt.Sprintf("research-prd-%d", time.Now().Unix())
	}
	return slug
}

// shellQuote wraps a string in single quotes for safe shell interpolation.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// execCommand runs a command and returns combined output.
func execCommand(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	return cmd.CombinedOutput()
}

// extractComparisonField extracts a comparison field (current_state, new_thing, improvement)
// from the details map. Handles both string values and nested "comparison" objects.
func extractComparisonField(details map[string]any, field string) string {
	if details == nil {
		return ""
	}
	// Try direct field first
	if v, ok := details[field].(string); ok && v != "" {
		return v
	}
	// Try nested under "comparison"
	if comp, ok := details["comparison"].(map[string]any); ok {
		if v, ok := comp[field].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

// loadKBContext loads relevant KB context for a given research type so the
// consultant knows what the system currently has. Includes live system state.
func (h *ResearchHandler) loadKBContext(ctx context.Context, researchType string) string {
	var sb strings.Builder

	// Inject live system state from generate_system_context.py
	if liveCtx := h.loadLiveSystemContext(); liveCtx != "" {
		sb.WriteString("## LIVE SYSTEM STATE\n\n")
		sb.WriteString("This is the current live state of the system, generated just now.\n")
		sb.WriteString("Use this to understand what is actually installed, running, and available.\n")
		sb.WriteString("Do NOT suggest anything that conflicts with these constraints.\n\n")
		sb.WriteString(liveCtx)
		sb.WriteString("\n\n")
	}

	// Then add KB context pack
	data, err := h.database.RPC(ctx, "kb_context_pack", map[string]any{
		"p_topic":  researchType,
		"p_limit":  10,
	})
	if err != nil || data == nil {
		return sb.String()
	}
	result := unwrapRPCResult(data, "kb_context_pack")
	var pack map[string]any
	if json.Unmarshal(result, &pack) != nil {
		return sb.String()
	}
	// Extract relevant sections
	sections, _ := pack["sections"].([]any)
	for _, sec := range sections {
		s, ok := sec.(map[string]any)
		if !ok {
			continue
		}
		sb.WriteString(fmt.Sprintf("## %s\n", getString(s, "title")))
		sb.WriteString(getString(s, "content"))
		sb.WriteString("\n\n")
	}
	context := sb.String()
	if len(context) > 8000 {
		context = context[:8000] + "\n...[truncated]"
	}
	return context
}

// loadLiveSystemContext runs the system context generator and returns the JSON output.
// Caches result for 5 minutes to avoid hammering the system.
var (
	liveContextCache     string
	liveContextCacheTime time.Time
	liveContextMu        sync.Mutex
)

func (h *ResearchHandler) loadLiveSystemContext() string {
	liveContextMu.Lock()
	defer liveContextMu.Unlock()

	// Cache for 5 minutes
	if time.Since(liveContextCacheTime) < 5*time.Minute && liveContextCache != "" {
		return liveContextCache
	}

	scriptPath := "/home/vibes/vibepilot/scripts/generate_system_context.py"
	ctx2, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx2, "python3", scriptPath)
	out, err := cmd.Output()
	if err != nil {
		log.Printf("[loadLiveSystemContext] Failed to run system context script: %v", err)
		return ""
	}

	result := string(out)
	if len(result) > 4000 {
		result = result[:4000] + "\n...[truncated]"
	}

	liveContextCache = result
	liveContextCacheTime = time.Now()
	log.Printf("[loadLiveSystemContext] Generated fresh system context (%d bytes)", len(result))
	return result
}

func setupResearchHandlers(
	ctx context.Context,
	router *runtime.EventRouter,
	factory *runtime.SessionFactory,
	pool *runtime.AgentPool,
	database db.Database,
	cfg *runtime.Config,
	connRouter *runtime.Router,
	usageTracker *runtime.UsageTracker,
	actionApplier *runtime.ResearchActionApplier,
) {
	handler := NewResearchHandler(database, factory, pool, connRouter, cfg, usageTracker, actionApplier)
	handler.Register(router)

	// Recover stale council_review items left from previous crash/restart
	handler.recoverStaleCouncilReviews(ctx)
}

// recoverStaleCouncilReviews finds research_suggestions stuck in council_review
// status for more than 10 minutes and re-triggers the council review process.
func (h *ResearchHandler) recoverStaleCouncilReviews(ctx context.Context) {
	stale, err := h.database.Query(ctx, "research_suggestions", map[string]any{
		"status": "council_review",
	})
	if err != nil {
		log.Printf("[ResearchRecovery] Failed to query stale council_review items: %v", err)
		return
	}

	var items []map[string]any
	if json.Unmarshal(stale, &items) != nil || len(items) == 0 {
		return
	}

	for _, item := range items {
		id, _ := item["id"].(string)
		title, _ := item["title"].(string)

		// Check if the item has been stale for more than 10 minutes
		updatedAt, _ := item["updated_at"].(string)
		if updatedAt == "" {
			continue
		}

		staleAge := 10 * time.Minute
		// Parse the timestamp (format varies by driver)
		var t time.Time
		for _, layout := range []string{
			"2006-01-02T15:04:05.999999999Z07:00",
			"2006-01-02T15:04:05Z07:00",
			"2006-01-02 15:04:05.999999999 -0700 MST",
			time.RFC3339Nano,
			time.RFC3339,
		} {
			if parsed, err := time.Parse(layout, updatedAt); err == nil {
				t = parsed
				break
			}
		}

		if t.IsZero() {
			log.Printf("[ResearchRecovery] Cannot parse timestamp for %s: %s", truncateID(id), updatedAt)
			continue
		}

		age := time.Since(t)
		if age < staleAge {
			log.Printf("[ResearchRecovery] %s (%s) still fresh (age: %s), skipping", truncateID(id), title, age)
			continue
		}

		log.Printf("[ResearchRecovery] Recovering stale council_review item: %s (%s), age: %s", truncateID(id), title, age)

		// Find or create the daily report for this item, then re-trigger council review
		findingsPath, _ := item["findings_path"].(string)
		detailsRaw, _ := item["details"]
		var details map[string]any
		if detailsRaw != nil {
			switch d := detailsRaw.(type) {
			case map[string]any:
				details = d
			default:
				details = map[string]any{}
			}
		}

		if findingsPath != "" {
			reportID := h.findOrCreateDailyReport(ctx, findingsPath, details)
			if reportID != "" {
				// Add this item to the report
				summary, _ := item["summary"].(string)
				findingType, _ := item["type"].(string)
				h.addReportItem(ctx, reportID, id, title, summary, findingType, details)

				// Check if the report is ready for review and route it
				h.checkAndRouteReport(ctx, reportID)
				log.Printf("[ResearchRecovery] Re-routed item %s to report %s", truncateID(id), truncateID(reportID))
			}
		} else {
			// No findings path -- reset to pending so it gets re-processed
			h.database.RPC(ctx, "update_research_suggestion_status", map[string]any{
				"p_id":     id,
				"p_status": "pending",
			})
			log.Printf("[ResearchRecovery] Reset item %s to pending (no findings_path)", truncateID(id))
		}
	}

	log.Printf("[ResearchRecovery] Scan complete, checked %d items", len(items))
}

// insertReviewItem adds a review item to the unified review queue.
func (h *ResearchHandler) insertReviewItem(ctx context.Context, itemType, sourceID, title, summary, priority string, payload ...map[string]any) {
	existing, err := h.database.Query(ctx, "review_items", map[string]any{
		"type":      itemType,
		"source_id": sourceID,
		"status":    "pending",
	})
	if err == nil {
		var items []map[string]any
		if json.Unmarshal(existing, &items) == nil && len(items) > 0 {
			log.Printf("[ResearchHandler] Skipping duplicate review_item: %s/%s already pending", itemType, truncateID(sourceID))
			return
		}
	}

	var payloadData map[string]any
	if len(payload) > 0 {
		payloadData = payload[0]
	} else {
		payloadData = map[string]any{}
	}

	_, err = h.database.Insert(ctx, "review_items", map[string]any{
		"type":     itemType,
		"source_id": sourceID,
		"title":    title,
		"summary":  summary,
		"payload":  payloadData,
		"status":   "pending",
		"priority": priority,
	})
	if err != nil {
		log.Printf("[ResearchHandler] Failed to insert review_item for %s %s: %v", itemType, sourceID, err)
	} else {
		log.Printf("[ResearchHandler] Review item created: %s/%s", itemType, truncateID(sourceID))
	}
}

// suggestionTitle fetches the title of a research suggestion by ID.
func suggestionTitle(ctx context.Context, database db.Database, id string) string {
	data, err := database.Query(ctx, "research_suggestions", map[string]any{
		"id":    id,
		"limit": 1,
	})
	if err != nil {
		return id
	}
	var rows []map[string]any
	if err := json.Unmarshal(data, &rows); err != nil || len(rows) == 0 {
		return id
	}
	if title, ok := rows[0]["title"].(string); ok {
		return title
	}
	return id
}

// writeReportDecisionDoc writes a formatted council decision document for a report.
func (h *ResearchHandler) writeReportDecisionDoc(
	ctx context.Context,
	title string,
	reportID string,
	findingsPath string,
	itemResults []reportItemResult,
) string {
	var sb strings.Builder

	sb.WriteString("# Council Decision: ")
	sb.WriteString(title)
	sb.WriteString("\n\n")
	sb.WriteString("**Report ID:** ")
	sb.WriteString(reportID)
	sb.WriteString("\n")
	sb.WriteString("**Date:** ")
	sb.WriteString(time.Now().Format("2006-01-02 15:04:05"))
	sb.WriteString("\n")
	sb.WriteString("**Items Reviewed:** ")
	sb.WriteString(fmt.Sprintf("%d", len(itemResults)))
	sb.WriteString("\n\n---\n\n")

	// Summary table
	sb.WriteString("## Summary\n\n")
	sb.WriteString("| # | Finding | Recommendation | Key Concern |\n")
	sb.WriteString("|---|---------|---------------|-------------|\n")
	for i, ir := range itemResults {
		concern := ""
		if len(ir.concerns) > 0 {
			concern = ir.concerns[0]
			if len(concern) > 60 {
				concern = concern[:57] + "..."
			}
		}
		sb.WriteString(fmt.Sprintf("| %d | %s | **%s** | %s |\n", i+1, ir.title, ir.recommendation, concern))
	}
	sb.WriteString("\n")

	// Per-item details
	sb.WriteString("## Item Details\n\n")
	for i, ir := range itemResults {
		sb.WriteString(fmt.Sprintf("### %d. %s\n\n", i+1, ir.title))
		sb.WriteString(fmt.Sprintf("**Council Recommendation:** %s\n\n", ir.recommendation))
		sb.WriteString(fmt.Sprintf("**Reasoning:** %s\n\n", ir.reasoning))

		if len(ir.councilVotes) > 0 {
			sb.WriteString("**Council Votes:**\n")
			for _, v := range ir.councilVotes {
				member, _ := v["member"].(float64)
				sb.WriteString(fmt.Sprintf("- Member %d: %s - %s\n",
					int(member),
					getString(v, "vote"),
					getString(v, "reasoning")))
			}
			sb.WriteString("\n")
		}

		if len(ir.concerns) > 0 {
			sb.WriteString("**Concerns:**\n")
			for _, c := range ir.concerns {
				sb.WriteString(fmt.Sprintf("- %s\n", c))
			}
			sb.WriteString("\n")
		}
	}

	sb.WriteString("## Your Decision\n\n");
	sb.WriteString("For each item above, choose:\n");
	sb.WriteString("- **Approve:** Include in PRD for implementation\n");
	sb.WriteString("- **Watch:** Revisit monthly for updates\n");
	sb.WriteString("- **Reject:** Close with reasoning\n\n");
	sb.WriteString("**Decide online:** Open this report in the [Knowledge Hub](https://graphs.vibestribe.rocks) Research section to use the interactive decision buttons.\n");

	// Derive decision doc path (strip knowledgebase/ prefix to avoid doubling)
	decisionPath := fmt.Sprintf("research/decisions/%s.md", reportID)
	if findingsPath != "" {
		clean := strings.TrimPrefix(findingsPath, "knowledgebase/")
		clean = strings.TrimPrefix(clean, "/")
		decisionPath = strings.Replace(clean, "research/", "research/decisions/", 1)
	}

	kbBase := "/home/vibes/knowledgebase"
	fullPath := kbBase + "/" + decisionPath
	dir := fullPath[:strings.LastIndex(fullPath, "/")]
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("[ReportCouncil] Failed to create dir %s: %v", dir, err)
		return ""
	}
	if err := os.WriteFile(fullPath, []byte(sb.String()), 0644); err != nil {
		log.Printf("[ReportCouncil] Failed to write decision doc %s: %v", fullPath, err)
		return ""
	}

	log.Printf("[ReportCouncil] Decision doc written to %s", decisionPath)
	return decisionPath
}
