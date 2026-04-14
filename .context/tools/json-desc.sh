#!/bin/bash
# .context/tools/json-desc.sh - Extract one-line description from JSON config
python3 -c '
import json, sys
f = sys.argv[1]
try:
    with open(f) as fh:
        d = json.load(fh)
except Exception as e:
    print("json: " + str(e))
    sys.exit(0)
for k in ["description", "name", "_comment"]:
    v = d.get(k)
    if isinstance(v, str) and len(v) > 0:
        print(v)
        sys.exit(0)
print("keys: " + ", ".join(list(d.keys())[:6]))
' "$1"
