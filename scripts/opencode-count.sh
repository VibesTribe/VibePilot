#!/bin/bash
# Check running MAIN opencode sessions (not language server subprocesses)

REAL_BINARY="/home/mjlockboxsocial/.opencode/bin/opencode-real"
count=0

echo "Active MAIN opencode sessions:"
for pid in $(pgrep -f "opencode" 2>/dev/null); do
    exe=$(readlink /proc/$pid/exe 2>/dev/null)
    if [ "$exe" = "$REAL_BINARY" ]; then
        ppid=$(grep PPid /proc/$pid/status 2>/dev/null | awk '{print $2}')
        parent_exe=$(readlink /proc/$ppid/exe 2>/dev/null)
        if [ "$parent_exe" != "$REAL_BINARY" ]; then
            count=$((count + 1))
            start_time=$(ps -p $pid -o lstart= 2>/dev/null | xargs)
            echo "  PID $pid - started $start_time"
        fi
    fi
done

echo ""
echo "Total: $count main session(s)"
