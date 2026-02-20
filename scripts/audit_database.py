#!/usr/bin/env python3
"""
VibePilot Database Audit Script

Comprehensive audit of the Supabase database state before autonomous testing.
This script:
1. Discovers all tables and their schemas
2. Identifies test/stale data that needs cleanup
3. Reports data quality issues
4. Provides cleanup recommendations

Run: python scripts/audit_database.py
"""

import os
import sys
from datetime import datetime, timedelta
from collections import defaultdict
from dotenv import load_dotenv
from supabase import create_client

load_dotenv()

SUPABASE_URL = os.getenv("SUPABASE_URL")
SUPABASE_KEY = os.getenv("SUPABASE_KEY")

if not SUPABASE_URL or not SUPABASE_KEY:
    print("ERROR: Missing SUPABASE_URL or SUPABASE_KEY")
    sys.exit(1)

db = create_client(SUPABASE_URL, SUPABASE_KEY)


def print_section(title):
    """Print a formatted section header."""
    print("\n" + "=" * 70)
    print(f" {title}")
    print("=" * 70)


def print_subsection(title):
    """Print a formatted subsection header."""
    print(f"\n--- {title} ---")


def discover_tables():
    """Discover all accessible tables in the database."""
    print_section("TABLE DISCOVERY")
    
    # Tables known to exist based on codebase analysis
    known_tables = [
        "projects",
        "tasks", 
        "task_packets",
        "task_runs",
        "task_backlog",
        "secrets_vault",
        "lessons_learned",
        "agent_tasks",
        "maintenance_commands",
        "rate_limit_windows",
        "model_performance",
        "session_states"
    ]
    
    found_tables = []
    missing_tables = []
    
    for table in known_tables:
        try:
            result = db.table(table).select("*", count="exact").limit(1).execute()
            count = result.count if hasattr(result, 'count') else "unknown"
            found_tables.append((table, count))
            print(f"  ✓ {table:30} (count: {count})")
        except Exception as e:
            missing_tables.append((table, str(e)))
            print(f"  ✗ {table:30} (not accessible)")
    
    return found_tables, missing_tables


def analyze_tasks():
    """Analyze the tasks table for data quality issues."""
    print_section("TASKS TABLE ANALYSIS")
    
    try:
        # Get all tasks
        result = db.table("tasks").select("*").execute()
        tasks = result.data or []
        
        print(f"Total tasks: {len(tasks)}")
        
        if not tasks:
            print("  No tasks found in database.")
            return
        
        # Analyze by status
        status_counts = defaultdict(int)
        for task in tasks:
            status = task.get("status", "unknown")
            status_counts[status] += 1
        
        print_subsection("Tasks by Status")
        for status, count in sorted(status_counts.items()):
            print(f"  {status:20}: {count:4d}")
        
        # Identify problematic statuses
        print_subsection("Potentially Problematic Tasks")
        
        # Tasks stuck in certain states
        problematic_states = ["in_progress", "claimed", "running", "error"]
        stuck_tasks = []
        
        for task in tasks:
            status = task.get("status", "")
            if status in problematic_states:
                # Check how long it's been in this state
                started = task.get("started_at")
                updated = task.get("updated_at") or task.get("created_at")
                
                stuck_tasks.append({
                    "id": task.get("id", "unknown")[:8],
                    "title": task.get("title", "Untitled")[:40],
                    "status": status,
                    "started": started,
                    "updated": updated,
                    "attempts": task.get("attempts", 0)
                })
        
        if stuck_tasks:
            print(f"  Tasks in active/stuck states: {len(stuck_tasks)}")
            for task in stuck_tasks[:10]:  # Show first 10
                print(f"    - {task['id']}... | {task['status']:15} | attempts: {task['attempts']} | {task['title']}")
            if len(stuck_tasks) > 10:
                print(f"    ... and {len(stuck_tasks) - 10} more")
        else:
            print("  No tasks stuck in active states.")
        
        # Identify test tasks
        print_subsection("Test Tasks (Potential Cleanup Targets)")
        test_tasks = []
        for task in tasks:
            title = task.get("title", "").lower()
            if any(kw in title for kw in ["test", "hello", "world", "example", "dummy"]):
                test_tasks.append({
                    "id": task.get("id", "unknown")[:8],
                    "title": task.get("title", "Untitled")[:50],
                    "status": task.get("status", "unknown")
                })
        
        if test_tasks:
            print(f"  Found {len(test_tasks)} test tasks:")
            for task in test_tasks:
                print(f"    - {task['id']}... | {task['status']:15} | {task['title']}")
        else:
            print("  No obvious test tasks found.")
        
        # Check for duplicate titles
        print_subsection("Duplicate Task Titles")
        titles = defaultdict(list)
        for task in tasks:
            title = task.get("title", "Untitled")
            titles[title].append(task.get("id"))
        
        duplicates = {t: ids for t, ids in titles.items() if len(ids) > 1}
        if duplicates:
            print(f"  Found {len(duplicates)} duplicate titles:")
            for title, ids in list(duplicates.items())[:5]:
                print(f"    '{title[:50]}' appears {len(ids)} times")
        else:
            print("  No duplicate titles found.")
        
        return tasks
        
    except Exception as e:
        print(f"  ERROR analyzing tasks: {e}")
        return []


def analyze_task_runs():
    """Analyze the task_runs table for data quality issues."""
    print_section("TASK_RUNS TABLE ANALYSIS")
    
    try:
        result = db.table("task_runs").select("*").execute()
        runs = result.data or []
        
        print(f"Total task runs: {len(runs)}")
        
        if not runs:
            print("  No task runs found in database.")
            return
        
        # Analyze by status
        status_counts = defaultdict(int)
        for run in runs:
            status = run.get("status", "unknown")
            status_counts[status] += 1
        
        print_subsection("Runs by Status")
        for status, count in sorted(status_counts.items()):
            print(f"  {status:15}: {count:4d}")
        
        # Token data analysis
        print_subsection("Token Data Quality")
        
        bad_token_data = []
        suspicious_runs = []
        
        for run in runs:
            tokens_used = run.get("tokens_used") or 0
            tokens_in = run.get("tokens_in") or 0
            tokens_out = run.get("tokens_out") or 0
            calculated = tokens_in + tokens_out
            
            # Runs where tokens_used doesn't match in+out
            if tokens_in > 0 or tokens_out > 0:
                if tokens_used != calculated:
                    bad_token_data.append({
                        "id": run.get("id", "unknown")[:8],
                        "model": run.get("model_id", "unknown"),
                        "old": tokens_used,
                        "new": calculated
                    })
            
            # Suspicious runs with high tokens_used but no in/out data
            if tokens_used > 1000 and tokens_in == 0 and tokens_out == 0:
                suspicious_runs.append({
                    "id": run.get("id", "unknown")[:8],
                    "model": run.get("model_id", "unknown"),
                    "tokens_used": tokens_used
                })
        
        print(f"  Runs with mismatched token counts: {len(bad_token_data)}")
        if bad_token_data:
            for run in bad_token_data[:5]:
                print(f"    - {run['id']}... ({run['model']}): {run['old']} -> {run['new']}")
        
        print(f"  Suspicious runs (high tokens, no in/out): {len(suspicious_runs)}")
        if suspicious_runs:
            for run in suspicious_runs[:5]:
                print(f"    - {run['id']}... ({run['model']}): {run['tokens_used']} tokens")
        
        # Model distribution
        print_subsection("Runs by Model")
        model_counts = defaultdict(int)
        for run in runs:
            model = run.get("model_id", "unknown")
            model_counts[model] += 1
        
        for model, count in sorted(model_counts.items(), key=lambda x: -x[1]):
            print(f"  {model:30}: {count:4d}")
        
        # Check for orphaned runs (no matching task)
        print_subsection("Orphaned Runs")
        task_ids = set()
        try:
            tasks_result = db.table("tasks").select("id").execute()
            task_ids = {t["id"] for t in (tasks_result.data or [])}
        except:
            pass
        
        orphaned = []
        for run in runs:
            task_id = run.get("task_id")
            if task_id and task_id not in task_ids:
                orphaned.append({
                    "id": run.get("id", "unknown")[:8],
                    "task_id": task_id[:8] if task_id else "none"
                })
        
        if orphaned:
            print(f"  Found {len(orphaned)} orphaned runs (no matching task):")
            for run in orphaned[:5]:
                print(f"    - run: {run['id']}... | task: {run['task_id']}...")
        else:
            print("  No orphaned runs found.")
        
        return runs
        
    except Exception as e:
        print(f"  ERROR analyzing task_runs: {e}")
        return []


def analyze_projects():
    """Analyze the projects table."""
    print_section("PROJECTS TABLE ANALYSIS")
    
    try:
        result = db.table("projects").select("*").execute()
        projects = result.data or []
        
        print(f"Total projects: {len(projects)}")
        
        for proj in projects:
            name = proj.get("name", "Unnamed")
            desc = proj.get("description", "")[:50]
            print(f"  - {name:30} | {desc}")
        
        return projects
        
    except Exception as e:
        print(f"  ERROR analyzing projects: {e}")
        return []


def analyze_other_tables():
    """Analyze other important tables."""
    print_section("OTHER TABLES ANALYSIS")
    
    tables_to_check = [
        ("secrets_vault", "API keys and secrets"),
        ("lessons_learned", "Learning records"),
        ("maintenance_commands", "Maintenance history"),
        ("rate_limit_windows", "Rate limiting data"),
        ("task_backlog", "Backlog items"),
        ("agent_tasks", "Agent task assignments")
    ]
    
    for table, description in tables_to_check:
        try:
            result = db.table(table).select("*", count="exact").limit(5).execute()
            count = result.count if hasattr(result, 'count') else len(result.data or [])
            print(f"  {table:25} ({description}): {count} records")
            
            # Show sample data if available
            if result.data:
                sample = result.data[0]
                keys = list(sample.keys())[:5]  # First 5 keys
                print(f"    Sample fields: {', '.join(keys)}")
                
        except Exception as e:
            print(f"  {table:25} ({description}): NOT ACCESSIBLE")


def generate_cleanup_recommendations(tasks, runs, projects):
    """Generate cleanup recommendations based on analysis."""
    print_section("CLEANUP RECOMMENDATIONS")
    
    recommendations = []
    
    # Check for test data
    if tasks:
        test_tasks = [t for t in tasks if any(kw in t.get("title", "").lower() 
                      for kw in ["test", "hello", "world", "example", "dummy"])]
        if test_tasks:
            recommendations.append({
                "priority": "HIGH",
                "action": "Archive or delete test tasks",
                "count": len(test_tasks),
                "reason": "Test tasks can confuse autonomous testing",
                "tables": ["tasks", "task_runs"]
            })
    
    # Check for stuck tasks
    if tasks:
        stuck = [t for t in tasks if t.get("status") in ["in_progress", "claimed", "running"]]
        if stuck:
            recommendations.append({
                "priority": "HIGH",
                "action": "Reset stuck tasks to 'available'",
                "count": len(stuck),
                "reason": "Tasks stuck in active states prevent new task processing",
                "tables": ["tasks"]
            })
    
    # Check for orphaned runs
    if runs and tasks:
        task_ids = {t["id"] for t in tasks}
        orphaned = [r for r in runs if r.get("task_id") not in task_ids]
        if orphaned:
            recommendations.append({
                "priority": "MEDIUM",
                "action": "Delete orphaned task_runs",
                "count": len(orphaned),
                "reason": "Runs without parent tasks are data orphans",
                "tables": ["task_runs"]
            })
    
    # Check for bad token data
    if runs:
        bad_tokens = []
        for run in runs:
            tokens_used = run.get("tokens_used") or 0
            tokens_in = run.get("tokens_in") or 0
            tokens_out = run.get("tokens_out") or 0
            if tokens_in > 0 or tokens_out > 0:
                if tokens_used != tokens_in + tokens_out:
                    bad_tokens.append(run)
        
        if bad_tokens:
            recommendations.append({
                "priority": "MEDIUM",
                "action": "Fix token data using cleanup_task_runs.py",
                "count": len(bad_tokens),
                "reason": "Token counts don't match in+out values",
                "tables": ["task_runs"]
            })
    
    # Check for old completed tasks that could be archived
    if tasks:
        old_completed = []
        cutoff = datetime.utcnow() - timedelta(days=30)
        for task in tasks:
            if task.get("status") == "completed" or task.get("status") == "merged":
                completed_at = task.get("completed_at")
                if completed_at:
                    try:
                        completed_date = datetime.fromisoformat(completed_at.replace("Z", "+00:00"))
                        if completed_date < cutoff:
                            old_completed.append(task)
                    except:
                        pass
        
        if old_completed:
            recommendations.append({
                "priority": "LOW",
                "action": "Archive old completed tasks",
                "count": len(old_completed),
                "reason": "Tasks older than 30 days can be archived for performance",
                "tables": ["tasks", "task_runs"]
            })
    
    # Print recommendations
    if recommendations:
        for i, rec in enumerate(recommendations, 1):
            print(f"\n{i}. [{rec['priority']}] {rec['action']}")
            print(f"   Count: {rec['count']} items")
            print(f"   Reason: {rec['reason']}")
            print(f"   Tables: {', '.join(rec['tables'])}")
    else:
        print("  No cleanup recommendations at this time.")
    
    return recommendations


def print_sql_queries():
    """Print useful SQL queries for manual inspection."""
    print_section("USEFUL SQL QUERIES FOR MANUAL INSPECTION")
    
    queries = [
        ("Count tasks by status", """
SELECT status, COUNT(*) 
FROM tasks 
GROUP BY status 
ORDER BY count DESC;
        """),
        ("Find stuck tasks (in_progress > 1 hour)", """
SELECT id, title, status, started_at, attempts
FROM tasks 
WHERE status IN ('in_progress', 'claimed', 'running')
  AND started_at < NOW() - INTERVAL '1 hour';
        """),
        ("Find tasks with high retry counts", """
SELECT id, title, status, attempts
FROM tasks 
WHERE attempts > 3
ORDER BY attempts DESC;
        """),
        ("Find orphaned task_runs", """
SELECT tr.id, tr.task_id, tr.status
FROM task_runs tr
LEFT JOIN tasks t ON tr.task_id = t.id
WHERE t.id IS NULL;
        """),
        ("Check token data consistency", """
SELECT id, model_id, tokens_used, tokens_in, tokens_out,
       (tokens_in + tokens_out) as calculated
FROM task_runs
WHERE tokens_used != (tokens_in + tokens_out)
  AND (tokens_in > 0 OR tokens_out > 0);
        """),
        ("Find test tasks", """
SELECT id, title, status, created_at
FROM tasks
WHERE LOWER(title) LIKE '%test%'
   OR LOWER(title) LIKE '%hello%'
   OR LOWER(title) LIKE '%example%';
        """),
        ("Task completion timeline", """
SELECT DATE(completed_at) as date, COUNT(*) as completed
FROM tasks
WHERE status = 'completed'
  AND completed_at > NOW() - INTERVAL '30 days'
GROUP BY DATE(completed_at)
ORDER BY date DESC;
        """),
    ]
    
    for name, sql in queries:
        print(f"\n{name}:")
        print("```sql")
        print(sql.strip())
        print("```")


def main():
    """Main audit function."""
    print("=" * 70)
    print(" VIBEPILOT DATABASE AUDIT")
    print(f" Started: {datetime.utcnow().isoformat()}")
    print("=" * 70)
    
    # Discover tables
    found_tables, missing_tables = discover_tables()
    
    # Analyze main tables
    tasks = analyze_tasks()
    runs = analyze_task_runs()
    projects = analyze_projects()
    
    # Analyze other tables
    analyze_other_tables()
    
    # Generate recommendations
    recommendations = generate_cleanup_recommendations(tasks, runs, projects)
    
    # Print useful queries
    print_sql_queries()
    
    # Summary
    print_section("AUDIT SUMMARY")
    print(f"Tables found: {len(found_tables)}")
    print(f"Tables missing: {len(missing_tables)}")
    print(f"Total tasks: {len(tasks) if tasks else 0}")
    print(f"Total task runs: {len(runs) if runs else 0}")
    print(f"Total projects: {len(projects) if projects else 0}")
    print(f"Cleanup recommendations: {len(recommendations)}")
    
    print("\n" + "=" * 70)
    print(" AUDIT COMPLETE")
    print("=" * 70)
    
    return {
        "found_tables": found_tables,
        "missing_tables": missing_tables,
        "tasks": tasks,
        "runs": runs,
        "projects": projects,
        "recommendations": recommendations
    }


if __name__ == "__main__":
    main()
