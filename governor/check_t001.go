package main

import (
    "context"
    "encoding/json"
    "log"
    "os"
    "github.com/vibepilot/governor/internal/db"
)

func main() {
    dbURL := os.Getenv("SUPABASE_URL")
    dbKey := os.Getenv("SUPABASE_SERVICE_KEY")
    
    if dbURL == "" || dbKey == "" {
        log.Fatal("Missing SUPABASE_URL or SUPABASE_SERVICE_KEY")
    }
    
    database := db.New(dbURL, dbKey)
    defer database.Close()
    
    ctx := context.Background()
    
    result, err := database.RPC(ctx, "get_task_by_number", map[string]any{
        "p_task_number": "T001",
    })
    if err != nil {
        log.Fatalf("Failed to get task: %v", err)
    }
    
    var task map[string]any
    if err := json.Unmarshal(result, &task); err != nil {
        log.Fatalf("Failed to parse task: %v", err)
    }
    
    log.Printf("Task T001 found:")
    log.Printf("  ID: %v", task["id"])
    log.Printf("  Status: %v", task["status"])
    log.Printf("  ProcessingBy: %v", task["processing_by"])
    log.Printf("  ProcessingAt: %v", task["processing_at"])
    log.Printf("  UpdatedAt: %v", task["updated_at"])
}
