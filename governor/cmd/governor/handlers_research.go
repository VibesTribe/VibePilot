package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
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

	_, err := h.database.Insert(ctx, "research_report_items", map[string]any{
		"report_id":     reportID,
		"suggestion_id": suggestionID,
		"title":         title,
		"finding_type":  findingType,
		"summary":       summary,
		"details":       detailsJSON,
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

	var report map[string]any
	if json.Unmarshal(reportData, &report) != nil {
		log.Printf("[ReportCouncil] Failed to parse report data")
		return
	}

	itemsRaw, _ := report["items"].([]any)
	if len(itemsRaw) == 0 {
		log.Printf("[ReportCouncil] Report %s has no items", truncateID(reportID))
		return
	}

	log.Printf("[ReportCouncil] Starting council review for report %s (%d items)", truncateID(reportID), len(itemsRaw))

	// Run council on the full report
	memberCount := h.cfg.GetCouncilMemberCount()
	lenses := h.cfg.GetCouncilLenses()
	if len(lenses) == 0 {
		lenses = []string{"user_alignment", "architecture", "feasibility"}
	}
	if memberCount <= 0 {
		memberCount = 3
	}

	results := make([]councilMemberResult, memberCount)
	var wg sync.WaitGroup
	var failedMembers []string
	var mu sync.Mutex

	for i := 0; i < memberCount; i++ {
		lens := lenses[i%len(lenses)]

		memberRouting, routeErr := h.connRouter.SelectRouting(ctx, runtime.RoutingRequest{
			Role:          "council",
			TaskType:      "research_council",
			RoutingFlag:   "internal",
			ExcludeModels: failedMembers,
		})
		if routeErr != nil || memberRouting == nil {
			log.Printf("[ReportCouncil] No routing for member %d, skipping", i+1)
			continue
		}

		session, err := h.factory.CreateWithConnector(ctx, "council", lens, memberRouting.ConnectorID)
		if err != nil {
			log.Printf("[ReportCouncil] Failed to create session for member %d: %v", i+1, err)
			mu.Lock()
			failedMembers = append(failedMembers, memberRouting.ModelID)
			mu.Unlock()
			continue
		}

		// Build context with all items for holistic review
		contextData := map[string]any{
			"report":       report,
			"items":        itemsRaw,
			"lens":         lens,
			"member_number": i + 1,
			"review_type":  "research_report",
		}

		wg.Add(1)
		go func(memberIndex int, sess *runtime.Session, routing *runtime.RoutingResult, memberLens string) {
			defer wg.Done()

			memberStart := time.Now()
			result, err := sess.Run(ctx, contextData)
			memberDuration := time.Since(memberStart).Seconds()
			if err != nil {
				log.Printf("[ReportCouncil] Member %d failed: %v", memberIndex+1, err)
				mu.Lock()
				failedMembers = append(failedMembers, routing.ModelID)
				mu.Unlock()
				if h.usageTracker != nil {
					h.usageTracker.RecordCompletion(ctx, routing.ModelID, "research_council", memberDuration, false)
				}
				results[memberIndex] = councilMemberResult{err: err}
				return
			}

			// Parse per-item votes from council output
			itemVotes := runtime.ParseReportCouncilVotes(result.Output)

			mu.Lock()
			results[memberIndex] = councilMemberResult{items: itemVotes}
			mu.Unlock()

			if h.usageTracker != nil {
				h.usageTracker.RecordCompletion(ctx, routing.ModelID, "research_council", memberDuration, true)
			}

			log.Printf("[ReportCouncil] Member %d (%s) completed review", memberIndex+1, memberLens)
		}(i, session, memberRouting, lens)
	}
	wg.Wait()

	// Aggregate per-item recommendations
	h.aggregateAndSaveCouncilResults(ctx, reportID, report, results, memberCount)

	log.Printf("[ReportCouncil] Report %s council review complete", truncateID(reportID))
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

		reasoning := "Council split."
		if len(agg.reasonings) > 0 {
			reasoning = agg.reasonings[0]
			if len(agg.reasonings) > 1 {
				reasoning += fmt.Sprintf(" (%d members voted)", len(agg.reasonings))
			}
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
	h.insertReviewItem(ctx, "research", reportID, reportTitle,
		fmt.Sprintf("Council reviewed %d items. Decision doc ready.", len(itemResults)),
		"high")

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
}

// insertReviewItem adds a review item to the unified review queue.
func (h *ResearchHandler) insertReviewItem(ctx context.Context, itemType, sourceID, title, summary, priority string) {
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

	_, err = h.database.Insert(ctx, "review_items", map[string]any{
		"type":     itemType,
		"source_id": sourceID,
		"title":    title,
		"summary":  summary,
		"payload":  map[string]any{},
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

	sb.WriteString("## Your Decision\n\n")
	sb.WriteString("For each item above, choose:\n")
	sb.WriteString("- **Approve:** Include in PRD for implementation\n")
	sb.WriteString("- **Watch:** Revisit monthly for updates\n")
	sb.WriteString("- **Reject:** Close with reasoning\n\n")
	sb.WriteString("Use the Review Hub to make your decisions.\n")

	// Derive decision doc path
	decisionPath := fmt.Sprintf("research/decisions/%s.md", reportID)
	if findingsPath != "" {
		decisionPath = strings.Replace(findingsPath, "research/", "research/decisions/", 1)
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
