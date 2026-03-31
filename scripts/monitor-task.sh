#!/bin/bash
# Real-time task monitoring with exact timings

LOG_FILE="$HOME/vibepilot/governor.log"
TASK_ID="${1:-}"

echo "=== VibePilot Real-Time Task Monitor ==="
echo "Monitoring: $TASK_ID (latest task if empty)"
echo "Press Ctrl+C to stop"
echo ""

# Track stages
declare -A stage_start
declare -A stage_end

tail -f "$LOG_FILE" | while read line; do
    # Extract timestamp
    timestamp=$(echo "$line" | grep -oP '\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}')
    
    # Detect stages
    if echo "$line" | grep -q "Processing plan.*status=draft"; then
        echo "[$timestamp] 🎯 STAGE 1: Plan Creation started"
    elif echo "$line" | grep -q "Plan.*created successfully in"; then
        duration=$(echo "$line" | grep -oP '\d+ms')
        echo "[$timestamp] ✅ STAGE 1: Plan created (${duration}ms)"
    elif echo "$line" | grep -q "Supervisor decision: approved"; then
        echo "[$timestamp] ✅ STAGE 2: Supervisor approved"
    elif echo "$line" | grep -q "Task.*claimed by"; then
        echo "[$timestamp] 🎯 STAGE 3: Task claimed by executor"
    elif echo "$line" | grep -q "Task.*decision: pass"; then
        echo "[$timestamp] ✅ STAGE 4: Review passed"
    elif echo "$line" | grep -q "Task.*outcome.*PASS"; then
        echo "[$timestamp] ✅ STAGE 5: Testing passed"
    elif echo "$line" | grep -q "Task.*merged to"; then
        branch=$(echo "$line" | grep -oP'merged to \K[^ ]+')
        echo "[$timestamp] 🎉 COMPLETE: Merged to $branch"
    elif echo "$line" | grep -q "Recovering stale task"; then
        echo "[$timestamp] ⚠️  WARNING: Task timeout, retrying..."
    fi
done
