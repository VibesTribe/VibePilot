#!/bin/bash
# Check running opencode sessions
count=$(ps aux | grep -c '[o]pencode')
echo "Active opencode sessions: $count / 5 max"
echo ""
ps aux | grep '[o]pencode' | awk '{printf "  PID %s - %s\n", $2, $9}'
