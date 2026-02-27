#!/bin/bash
# Cleanup zombie AI processes that accumulate over time
# Run via cron: 0 * * * * /home/mjlockboxsocial/vibepilot/scripts/cleanup_zombies.sh
#
# IMPORTANT: This script checks cgroup membership to avoid killing
# governor-spawned processes. Governor children are in the
# vibepilot-governor.service cgroup and should NOT be killed here.

echo "$(date): Cleaning up zombie processes..."

# Check if a process is in the governor's cgroup (should be protected)
is_in_governor_cgroup() {
    local pid=$1
    local cgroup=$(cat /proc/$pid/cgroup 2>/dev/null | head -1)
    [[ "$cgroup" == *"vibepilot-governor.service"* ]]
}

# Cleanup opencode processes that are truly orphaned zombies
# Kill only if:
# 1. NOT in governor's cgroup (governor manages those)
# 2. NOT connected to a terminal (tty=?)
# 3. Orphaned (PPID=1) - parent died
ps aux | grep -E "[o]pencode" | while read line; do
    pid=$(echo $line | awk '{print $2}')
    
    # Skip if in governor's cgroup
    if is_in_governor_cgroup "$pid"; then
        continue
    fi
    
    tty=$(ps -o tty= -p $pid 2>/dev/null | tr -d ' ')
    ppid=$(ps -o ppid= -p $pid 2>/dev/null | tr -d ' ')
    
    # Skip if connected to terminal (active user session)
    if [[ "$tty" != "?" ]] && [[ -n "$tty" ]]; then
        continue
    fi
    
    # Kill only if orphaned (parent died, adopted by init)
    if [[ "$ppid" == "1" ]]; then
        echo "Killing orphan opencode PID $pid (ppid=1, no terminal)"
        kill -TERM $pid 2>/dev/null || kill -KILL $pid 2>/dev/null
    fi
done

# Cleanup kimi processes - same logic
ps aux | grep -E "[k]imi" | while read line; do
    pid=$(echo $line | awk '{print $2}')
    
    # Skip if in governor's cgroup
    if is_in_governor_cgroup "$pid"; then
        continue
    fi
    
    tty=$(ps -o tty= -p $pid 2>/dev/null | tr -d ' ')
    ppid=$(ps -o ppid= -p $pid 2>/dev/null | tr -d ' ')
    
    # Skip if connected to terminal
    if [[ "$tty" != "?" ]] && [[ -n "$tty" ]]; then
        continue
    fi
    
    # Kill only if orphaned
    if [[ "$ppid" == "1" ]]; then
        echo "Killing orphan kimi PID $pid (ppid=1, no terminal)"
        kill -TERM $pid 2>/dev/null || kill -KILL $pid 2>/dev/null
    fi
done

echo "$(date): Cleanup complete"
