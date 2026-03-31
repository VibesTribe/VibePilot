#!/bin/bash
# Extract timing metrics from governor log

LOG_FILE="$HOME/vibepilot/governor.log"
OUTPUT="$HOME/vibepilot/timing_report.txt"

echo "=== VibePilot Timing Report ===" > "$OUTPUT"
echo "Generated: $(date)" >> "$OUTPUT"
echo "" >> "$OUTPUT"

# Extract task timings
grep -E "Processing plan|Plan.*created successfully|Supervisor decision|Task.*claimed|Task.*outcome|Task.*merged" "$LOG_FILE" | tail -50 >> "$OUTPUT"

echo "" >> "$OUTPUT"
echo "=== End-to-End Timings ===" >> "$OUTPUT"

# Calculate overall timing
grep -E "Plan created|Task.*merged" "$LOG_FILE" | tail -10 >> "$OUTPUT"

cat "$OUTPUT"
echo ""
echo "Report saved to: $OUTPUT"
