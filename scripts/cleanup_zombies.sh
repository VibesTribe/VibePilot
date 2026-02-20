#!/bin/bash
# Cleanup zombie AI processes that accumulate over time
# Run via cron: 0 * * * * /home/mjlockboxsocial/vibepilot/scripts/cleanup_zombies.sh

echo "$(date): Cleaning up zombie processes..."

# Kill opencode processes older than 6 hours
ps aux | grep opencode | grep -v grep | while read line; do
    pid=$(echo $line | awk '{print $2}')
    start_time=$(echo $line | awk '{print $9}')
    
    # Check if start time indicates old process (format varies)
    # Kill if from previous day or older than 6 hours
    if [[ "$start_time" =~ ^Feb ]] || [[ "$start_time" < "$(date -d '6 hours ago' +%H:%M 2>/dev/null || echo '00:00')" ]]; then
        # Don't kill if it's the current active session (connected to terminal)
        tty=$(ps -o tty= -p $pid 2>/dev/null | tr -d ' ')
        if [[ "$tty" == "?" ]] || [[ -z "$tty" ]]; then
            echo "Killing zombie opencode PID $pid (started: $start_time)"
            kill -TERM $pid 2>/dev/null || kill -KILL $pid 2>/dev/null
        fi
    fi
done

# Kill old kimi processes (older than 6 hours, not connected to terminal)
ps aux | grep "kimi" | grep -v grep | grep -v grep | while read line; do
    pid=$(echo $line | awk '{print $2}')
    start_time=$(echo $line | awk '{print $9}')
    tty=$(ps -o tty= -p $pid 2>/dev/null | tr -d ' ')
    
    if [[ "$start_time" =~ ^Feb ]] || [[ "$tty" == "?" ]]; then
        echo "Killing zombie kimi PID $pid"
        kill -TERM $pid 2>/dev/null || kill -KILL $pid 2>/dev/null
    fi
done

echo "$(date): Cleanup complete"
