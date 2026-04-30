#!/usr/bin/env python3
"""
Audit Supabase schema, Go code, and dashboard to build a consolidated
traceability map. Outputs to knowledge.db schema_current table.

This replays all SQL migrations to determine the CURRENT state of every table,
then cross-references with Go code field usage and dashboard field expectations.
"""
import re
import os
import sqlite3
from pathlib import Path
from collections import defaultdict

REPO_ROOT = Path(__file__).resolve().parent.parent.parent
SCHEMA_DIR = REPO_ROOT / "docs" / "supabase-schema"
GOVERNOR_DIR = REPO_ROOT / "governor"
DB_PATH = REPO_ROOT / ".context" / "knowledge.db"


def migration_sort_key(f):
    """Sort migration files in execution order: schema base first, then numbered, then fixes."""
    name = f.stem
    if name == "schema_v1_core":
        return (0, 0, name)
    if name.startswith("schema_v1"):
        return (0, 1, name)
    if name.startswith("schema_"):
        return (0, 2, name)
    m = re.match(r"^(\d+)", name)
    if m:
        return (1, int(m.group(1)), name)
    return (2, 0, name)


def parse_migrations():
    """Replay all SQL migrations to build current table state."""
    tables = {}  # table_name -> {col_name: col_def, ...}
    functions = {}  # func_name -> signature
    indexes = {}  # index_name -> definition
    types = {}  # type_name -> definition
    
    for sql_file in sorted(SCHEMA_DIR.glob("*.sql"), key=migration_sort_key):
        text = sql_file.read_text(errors="replace")
        migration = sql_file.stem
        
        # Process CREATE TABLE and DROP TABLE in file order
        # First collect all positions and types
        events = []
        for match in re.finditer(r"CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?(\w+)\s*\(", text, re.IGNORECASE):
            events.append((match.start(), "create", match))
        for match in re.finditer(r"DROP\s+TABLE\s+(?:IF\s+EXISTS\s+)?(\w+)", text, re.I):
            events.append((match.start(), "drop", match))
        events.sort(key=lambda x: x[0])
        
        for pos, event_type, match in events:
            if event_type == "drop":
                tables.pop(match.group(1), None)
            else:
                table_name = match.group(1)
                start = match.end()
                depth = 1
                p = start
                while p < len(text) and depth > 0:
                    if text[p] == '(':
                        depth += 1
                    elif text[p] == ')':
                        depth -= 1
                    p += 1
                body = text[start:p-1]
                if table_name not in tables:
                    tables[table_name] = {"_migration_created": migration}
                for line in body.split("\n"):
                    line = line.strip().rstrip(",")
                    if not line or line.startswith("--"):
                        continue
                    if re.match(r"^(PRIMARY|FOREIGN|UNIQUE|CHECK|CONSTRAINT|EXCLUDE)", line, re.I):
                        continue
                    col_match = re.match(r"(\w+)\s+(.+)", line)
                    if col_match:
                        tables[table_name][col_match.group(1)] = col_match.group(2).strip()
        
        # Similarly for DROP/CREATE FUNCTION in file order
        func_events = []
        for match in re.finditer(r"CREATE\s+(?:OR\s+REPLACE\s+)?FUNCTION\s+(\w+)\s*\(([^)]*)\)", text, re.IGNORECASE):
            func_events.append((match.start(), "create", match))
        for match in re.finditer(r"DROP\s+FUNCTION\s+(?:IF\s+EXISTS\s+)?(\w+)", text, re.I):
            func_events.append((match.start(), "drop", match))
        func_events.sort(key=lambda x: x[0])
        
        for pos, event_type, match in func_events:
            if event_type == "drop":
                functions.pop(match.group(1), None)
            else:
                func_name = match.group(1)
                args = match.group(2).strip()
                ret_match = re.search(r"RETURNS\s+(\S+)", text[match.end():match.end()+200])
                ret_type = ret_match.group(1) if ret_match else "?"
                functions[func_name] = f"({args}) RETURNS {ret_type}"
        
        # Parse ALTER TABLE ADD COLUMN (single and multi-line)
        # Multi-line: ALTER TABLE foo ADD COLUMN IF NOT EXISTS col1 type, col2 type, ... ;
        for match in re.finditer(
            r"ALTER\s+TABLE\s+(\w+)\s+ADD\s+COLUMN\s+IF\s+NOT\s+EXISTS\s*\n?\s*(.*?)\s*\);",
            text, re.IGNORECASE | re.DOTALL
        ):
            table_name = match.group(1)
            body = match.group(2)
            if table_name not in tables:
                tables[table_name] = {"_migration_created": migration}
            for line in body.split(","):
                line = line.strip().rstrip(",")
                if not line:
                    continue
                col_match = re.match(r"(\w+)\s+(.+)", line)
                if col_match:
                    col_name = col_match.group(1)
                    col_def = col_match.group(2).strip()
                    tables[table_name][col_name] = col_def
        
        # Single-line ALTER TABLE ADD COLUMN
        for match in re.finditer(
            r"ALTER\s+TABLE\s+(\w+)\s+ADD\s+COLUMN\s+(?:IF\s+NOT\s+EXISTS\s+)?(\w+)\s+([^,;]+)",
            text, re.IGNORECASE
        ):
            table_name = match.group(1)
            col_name = match.group(2)
            col_def = match.group(3).strip()
            if table_name not in tables:
                tables[table_name] = {"_migration_created": migration}
            tables[table_name][col_name] = col_def
        
        # Parse ALTER TABLE ALTER COLUMN
        for match in re.finditer(
            r"ALTER\s+TABLE\s+(\w+)\s+ALTER\s+COLUMN\s+(\w+)\s+(.+?)(?:;|\n)",
            text, re.IGNORECASE
        ):
            table_name = match.group(1)
            col_name = match.group(2)
            alteration = match.group(3).strip()
            if table_name in tables and col_name in tables[table_name]:
                tables[table_name][col_name] += f" | {alteration}"
        
        # Parse CREATE INDEX
        for match in re.finditer(
            r"CREATE\s+(?:UNIQUE\s+)?INDEX\s+(?:IF\s+NOT\s+EXISTS\s+)?(\w+)\s+ON\s+(\w+)\s*\(([^)]+)\)",
            text, re.IGNORECASE
        ):
            idx_name = match.group(1)
            idx_table = match.group(2)
            idx_cols = match.group(3)
            indexes[idx_name] = f"ON {idx_table}({idx_cols})"
        
        # Parse CREATE TYPE
        for match in re.finditer(
            r"CREATE\s+TYPE\s+(\w+)\s+AS\s+ENUM\s*\(([^)]+)\)",
            text, re.IGNORECASE
        ):
            types[match.group(1)] = f"ENUM({match.group(2)})"
    
    return tables, functions, indexes, types


def audit_go_code():
    """Find what fields Go code reads/writes from each table."""
    field_usage = defaultdict(lambda: defaultdict(set))
    
    go_files = list(GOVERNOR_DIR.rglob("*.go"))
    for go_file in go_files:
        if "vendor" in str(go_file) or "_test.go" in str(go_file):
            continue
        try:
            text = go_file.read_text(errors="replace")
        except:
            continue
        
        # RPC calls that write to tables
        for match in re.finditer(r'\.RPC\(ctx,\s*"(\w+)"', text):
            rpc = match.group(1)
            field_usage["rpc_calls"][rpc].add(go_file.name)
        
        # Direct field access: task["field_name"], task["field"]
        for match in re.finditer(r'(\w+)\["(\w+)"\]', text):
            var = match.group(1)
            field = match.group(2)
            if var in ("task", "plan", "result", "review", "test_output", "decision", "packet"):
                field_usage[f"go_reads_{var}"][field].add(go_file.name)
        
        # Map building: "field": value
        for match in re.finditer(r'"(\w+)":\s*\w+', text):
            field = match.group(1)
            if field.startswith("p_"):
                field_usage["go_rpc_params"][field].add(go_file.name)
        
        # Table references in comments and queries
        for match in re.finditer(r'(?:FROM|INTO|UPDATE|DELETE FROM)\s+(\w+)', text, re.I):
            table = match.group(1).lower()
            if table not in ("select", "null", "true", "false", "now"):
                field_usage["go_table_refs"][table].add(go_file.name)
    
    return field_usage


def audit_dashboard():
    """Find what fields the dashboard expects from Supabase tables."""
    dashboard_dir = Path.home() / "vibeflow"
    if not dashboard_dir.exists():
        return {}
    
    field_usage = defaultdict(lambda: defaultdict(set))
    
    # Search React/TypeScript files for field references
    for ext in ("*.tsx", "*.ts", "*.jsx", "*.js"):
        for ts_file in dashboard_dir.rglob(ext):
            if "node_modules" in str(ts_file) or ".next" in str(ts_file):
                continue
            try:
                text = ts_file.read_text(errors="replace")
            except:
                continue
            
            # Direct field access: .field_name, ["field_name"]
            for match in re.finditer(r'\.(\w+)\b', text):
                field = match.group(1)
                if len(field) > 2 and not field.startswith("__"):
                    field_usage["dashboard_fields"][field].add(ts_file.name)
            
            # Table references: from('table'), .from('table')
            for match in re.finditer(r"from\(['\"](\w+)['\"]\)", text):
                table = match.group(1)
                field_usage["dashboard_tables"][table].add(ts_file.name)
    
    return field_usage


def build_consolidated_view(tables, functions, go_usage, dash_usage):
    """Build a cross-referenced view of schema ↔ Go ↔ Dashboard."""
    rows = []
    
    for table_name, columns in sorted(tables.items()):
        # Get Go files that reference this table
        go_files = go_usage.get("go_table_refs", {}).get(table_name, set())
        dash_files = dash_usage.get("dashboard_tables", {}).get(table_name, set())
        
        real_cols = {k: v for k, v in columns.items() if not k.startswith("_")}
        
        for col_name, col_def in sorted(real_cols.items()):
            # Check if Go reads this field
            go_readers = set()
            for var in ("task", "plan", "result", "review"):
                if col_name in go_usage.get(f"go_reads_{var}", {}):
                    go_readers.update(go_usage[f"go_reads_{var}"][col_name])
            
            # Check if dashboard uses this field
            dash_readers = dash_usage.get("dashboard_fields", {}).get(col_name, set())
            
            # Determine status
            in_go = "yes" if go_readers else ""
            in_dash = "yes" if dash_readers else ""
            source = "active" if (go_readers or dash_files) else "schema_only"
            
            rows.append({
                "table_name": table_name,
                "column_name": col_name,
                "type_definition": col_def.split("--")[0].strip(),
                "used_in_go": in_go,
                "go_files": ",".join(sorted(go_readers)) if go_readers else None,
                "used_in_dashboard": in_dash,
                "dash_files": ",".join(sorted(dash_readers)[:3]) if dash_readers else None,
                "status": source,
            })
    
    return rows


def main():
    print("[audit] Parsing 111 SQL migrations...")
    tables, functions, indexes, types = parse_migrations()
    print(f"[audit] Found {len(tables)} tables, {len(functions)} functions, {len(indexes)} indexes, {len(types)} types")
    
    print("[audit] Scanning Go code...")
    go_usage = audit_go_code()
    print(f"[audit] Go: {len(go_usage.get('rpc_calls', {}))} RPC calls, {len(go_usage.get('go_reads_task', {}))} task fields read")
    
    print("[audit] Scanning dashboard...")
    dash_usage = audit_dashboard()
    print(f"[audit] Dashboard: {len(dash_usage.get('dashboard_tables', {}))} tables referenced")
    
    print("[audit] Building consolidated view...")
    rows = build_consolidated_view(tables, functions, go_usage, dash_usage)
    print(f"[audit] {len(rows)} table-column pairs")
    
    # Write to knowledge.db
    conn = sqlite3.connect(str(DB_PATH))
    c = conn.cursor()
    
    c.execute("DROP TABLE IF EXISTS schema_current")
    c.execute("""CREATE TABLE schema_current (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        table_name TEXT NOT NULL,
        column_name TEXT NOT NULL,
        type_definition TEXT,
        used_in_go TEXT,
        go_files TEXT,
        used_in_dashboard TEXT,
        dash_files TEXT,
        status TEXT
    )""")
    
    c.execute("CREATE INDEX idx_sc_table ON schema_current(table_name)")
    c.execute("CREATE INDEX idx_sc_status ON schema_current(status)")
    
    for row in rows:
        c.execute("INSERT INTO schema_current (table_name, column_name, type_definition, used_in_go, go_files, used_in_dashboard, dash_files, status) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
                  (row["table_name"], row["column_name"], row["type_definition"], row["used_in_go"], row["go_files"], row["used_in_dashboard"], row["dash_files"], row["status"]))
    
    # Also store functions, indexes, types as reference
    c.execute("DROP TABLE IF EXISTS schema_functions")
    c.execute("""CREATE TABLE schema_functions (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        name TEXT NOT NULL,
        signature TEXT,
        called_by_go TEXT
    )""")
    c.execute("CREATE INDEX idx_sf_name ON schema_functions(name)")
    
    for name, sig in sorted(functions.items()):
        go_callers = go_usage.get("rpc_calls", {}).get(name, set())
        c.execute("INSERT INTO schema_functions (name, signature, called_by_go) VALUES (?, ?, ?)",
                  (name, sig, ",".join(sorted(go_callers)) if go_callers else None))
    
    # Store metadata
    c.execute("DELETE FROM meta WHERE key LIKE 'audit_%'")
    c.execute("INSERT INTO meta VALUES ('audit_tables', ?)", (str(len(tables)),))
    c.execute("INSERT INTO meta VALUES ('audit_functions', ?)", (str(len(functions)),))
    c.execute("INSERT INTO meta VALUES ('audit_columns', ?)", (str(len(rows)),))
    c.execute("INSERT INTO meta VALUES ('audit_indexes', ?)", (str(len(indexes)),))
    c.execute("INSERT INTO meta VALUES ('audit_types', ?)", (str(len(types)),))
    
    conn.commit()
    
    # Print summary
    print(f"\n[audit] === CONSOLIDATED SCHEMA ===")
    print(f"[audit] Tables: {len(tables)}")
    print(f"[audit] Total columns: {len(rows)}")
    print(f"[audit] Functions: {len(functions)}")
    print(f"[audit] Indexes: {len(indexes)}")
    print(f"[audit] Custom types: {len(types)}")
    
    # Show tables with column counts
    print(f"\n[audit] === TABLES ===")
    table_counts = defaultdict(int)
    for r in rows:
        table_counts[r["table_name"]] += 1
    for t, count in sorted(table_counts.items(), key=lambda x: -x[1]):
        go_refs = len(go_usage.get("go_table_refs", {}).get(t, set()))
        dash_refs = len(dash_usage.get("dashboard_tables", {}).get(t, set()))
        print(f"  {t}: {count} cols | Go:{go_refs} Dash:{dash_refs}")
    
    # Show Go RPC calls
    print(f"\n[audit] === Go RPC CALLS ===")
    for rpc, files in sorted(go_usage.get("rpc_calls", {}).items()):
        print(f"  {rpc} <- {','.join(sorted(files))}")
    
    conn.close()
    print(f"\n[audit] Written to {DB_PATH}")


if __name__ == "__main__":
    main()
