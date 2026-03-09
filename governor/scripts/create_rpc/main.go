package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL not set")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer pool.Close()

	sql := `
CREATE OR REPLACE FUNCTION create_task_if_not_exists(
    p_plan_id UUID,
    p_task_number TEXT,
    p_title TEXT,
    p_type TEXT DEFAULT 'feature',
    p_status TEXT DEFAULT 'available',
    p_priority INT DEFAULT 5,
    p_confidence FLOAT DEFAULT 0.95,
    p_category TEXT DEFAULT 'coding',
    p_routing_flag TEXT DEFAULT NULL,
    p_routing_flag_reason TEXT DEFAULT NULL,
    p_dependencies JSONB DEFAULT '[]'::jsonb,
    p_prompt TEXT DEFAULT NULL,
    p_expected_output TEXT DEFAULT NULL,
    p_context JSONB DEFAULT '{}'::jsonb,
    p_max_attempts INT DEFAULT 3
) RETURNS UUID AS $$
DECLARE v_task_id UUID;
BEGIN
    INSERT INTO tasks (
        plan_id, task_number, title, type, status, priority,
        confidence, category, routing_flag, routing_flag_reason,
        dependencies, max_attempts, created_at, updated_at
    ) VALUES (
        p_plan_id, p_task_number, p_title, p_type, p_status, p_priority,
        p_confidence, p_category, p_routing_flag, p_routing_flag_reason,
        p_dependencies, p_max_attempts, NOW(), NOW()
    )
    ON CONFLICT (plan_id, task_number) DO NOTHING
    RETURNING id INTO v_task_id;
    
    IF v_task_id IS NOT NULL AND p_prompt IS NOT NULL THEN
        INSERT INTO task_packets (
            task_id, prompt, expected_output, context, version, created_at
        ) VALUES (
            v_task_id, p_prompt, p_expected_output, p_context, 1, NOW()
        );
    END IF;
    
    RETURN v_task_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;
`

	_, err = pool.Exec(ctx, sql)
	if err != nil {
		log.Fatalf("Failed to create function: %v", err)
	}

	fmt.Println("Function created successfully")
}
