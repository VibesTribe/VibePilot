package handlers

import (
	"context"
	"encoding/json"
	"log"

	"github.com/vibepilot/governor/internal/db"
	"github.com/vibepilot/governor/internal/runtime"
)

type TaskCreatedHandler struct{}
type PlanCreatedHandler struct{}
type PlanApprovedHandler struct{}
type PlanReviewHandler struct{}
type CouncilReviewHandler struct{}
type CouncilDoneHandler struct{}
type TestResultsHandler struct{}
type MaintenanceHandler struct{}
type ResearchHandler struct{}
type PRDReadyHandler struct{}

type Handlers struct {
	task     TaskCreatedHandler
    plan    PlanCreatedHandler
    approve PlanApprovedHandler
    review PlanReviewHandler
    council CouncilReviewHandler
    councilDone CouncilDoneHandler
    test   TestResultsHandler
    maint  MaintenanceHandler
    research ResearchHandler
    prd    PRDReadyHandler
}

func Handle(eventType string, data []byte) runtime.EventHandler {
    h, ok := handlers[eventType]
    if !ok {
        log.Printf("[Webhooks] Unknown event type: %s", eventType)
 "available handlers: %+v, eventType)
    return
    handler(event)
}
