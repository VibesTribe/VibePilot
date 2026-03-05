#!/bin/bash
# Check running MAIN kilo sessions

count=0

echo "Active MAIN kilo sessions:"
for pid in $(pgrep -f "\.kilo" 2>/dev/null); do
    exe=$(readlink /proc/$pid/exe 2>/dev/null)
    if [[ "$exe" == *"node"* ]]; then
        cmdline=$(tr '\0' ' ' < /proc/$pid/cmdline 2>/dev/null)
        if [[ "$cmdline" == *".kilo"* ]]; then
            ppid=$(grep PPid /proc/$pid/status 2>/dev/null | awk '{print $2}')
            parent_cmdline=$(tr '\0' ' ' < /proc/$ppid/cmdline 2>/dev/null)
            if [[ "$parent_cmdline" != *".kilo"* ]]; then
                count=$((count + 1))
                start_time=$(ps -p $pid -o lstart= 2>/dev/null | xargs)
                mem=$(ps -p $pid -o rss= 2>/dev/null | awk '{printf "%.0f MB", $1/1024}')
                echo "  PID $pid - started $start_time - $mem"
            fi
        fi
    fi
done

echo ""
echo "Total: $count main session(s)"
