import os
from supabase import create_client
from dotenv import load_dotenv

load_dotenv()
client = create_client(os.getenv("SUPABASE_URL"), os.getenv("SUPABASE_KEY"))

def scan_directory(root_path="."):
    structure = []
    for root, dirs, files in os.walk(root_path):
        dirs[:] = [d for d in dirs if not d.startswith('.') and d != 'venv' and d != 'node_modules']
        for file in files:
            full_path = os.path.join(root, file)
            rel_path = full_path.replace("./", "")
            deps = []
            if file.endswith(".py"):
                try:
                    with open(full_path, 'r') as f:
                        lines = [l for l in f.read().split('\n') if l.startswith('import') or l.startswith('from')]
                        deps = lines
                except: pass
            structure.append({
                "file_path": rel_path, "file_type": "file", "parent_path": os.path.dirname(rel_path) or ".", "dependencies": deps, "is_directory": False
            })
        for dir in dirs:
            rel_path = os.path.join(root, dir).replace("./", "")
            structure.append({
                "file_path": rel_path, "file_type": "dir", "parent_path": os.path.dirname(rel_path) or ".", "dependencies": [], "is_directory": True
            })
    return structure

def main():
    print("🗺️  Updating Project Map...")
    files = scan_directory(".")
    for item in files:
        try: client.table('project_structure').upsert(item).execute()
        except: pass
    print("✅ Map Updated.")

if __name__ == "__main__": main()
