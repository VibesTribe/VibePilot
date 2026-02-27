"""
VibePilot Memory Interface

Pluggable memory system for context storage and retrieval.
Current implementation: File-based (reads documents)
Future implementations: Vector DB, Graph RAG, Superseding tech

Usage:
    from core.memory import Memory

    memory = Memory()

    # Store something
    memory.store(project_id, "decision", {...}, {"type": "architecture"})

    # Retrieve by key
    decision = memory.retrieve(project_id, "decision")

    # Search (future: semantic, current: keyword)
    results = memory.search(project_id, "authentication")
"""

from abc import ABC, abstractmethod
from typing import Dict, Any, List, Optional
from datetime import datetime
import json
import os
import logging

logger = logging.getLogger("VibePilot.Memory")


class MemoryBackend(ABC):
    """Abstract base class for memory backends. Implement this to add new storage."""

    @abstractmethod
    def store(
        self, project_id: str, key: str, content: Any, metadata: Dict = None
    ) -> bool:
        """Store content with key under project."""
        pass

    @abstractmethod
    def retrieve(self, project_id: str, key: str) -> Optional[Any]:
        """Retrieve content by key."""
        pass

    @abstractmethod
    def search(
        self, project_id: str, query: str, limit: int = 10, filters: Dict = None
    ) -> List[Dict]:
        """Search for content matching query."""
        pass

    @abstractmethod
    def delete(self, project_id: str, key: str) -> bool:
        """Delete content by key."""
        pass

    @abstractmethod
    def list_keys(self, project_id: str, prefix: str = None) -> List[str]:
        """List all keys under project, optionally filtered by prefix."""
        pass


class FileBackend(MemoryBackend):
    """
    File-based memory backend.

    Storage location: data/memory/{project_id}/{key}.json
    This is the default - simple, portable, no dependencies.

    For production scale, swap to VectorBackend or GraphBackend.
    """

    def __init__(self, base_path: str = None):
        self.base_path = base_path or os.path.join(
            os.path.dirname(__file__), "..", "data", "memory"
        )
        os.makedirs(self.base_path, exist_ok=True)

    def _project_path(self, project_id: str) -> str:
        path = os.path.join(self.base_path, project_id)
        os.makedirs(path, exist_ok=True)
        return path

    def _key_path(self, project_id: str, key: str) -> str:
        safe_key = key.replace("/", "__").replace("\\", "__")
        return os.path.join(self._project_path(project_id), f"{safe_key}.json")

    def store(
        self, project_id: str, key: str, content: Any, metadata: Dict = None
    ) -> bool:
        try:
            data = {
                "key": key,
                "content": content,
                "metadata": metadata or {},
                "stored_at": datetime.utcnow().isoformat(),
                "backend": "file",
            }

            path = self._key_path(project_id, key)
            with open(path, "w") as f:
                json.dump(data, f, indent=2, default=str)

            logger.debug(f"Stored {key} for project {project_id}")
            return True
        except Exception as e:
            logger.error(f"Failed to store {key}: {e}")
            return False

    def retrieve(self, project_id: str, key: str) -> Optional[Any]:
        try:
            path = self._key_path(project_id, key)
            if not os.path.exists(path):
                return None

            with open(path, "r") as f:
                data = json.load(f)

            return data.get("content")
        except Exception as e:
            logger.error(f"Failed to retrieve {key}: {e}")
            return None

    def search(
        self, project_id: str, query: str, limit: int = 10, filters: Dict = None
    ) -> List[Dict]:
        """
        Keyword search (basic implementation).
        Future backends will do semantic search.
        """
        results = []
        query_lower = query.lower()

        project_path = self._project_path(project_id)
        if not os.path.exists(project_path):
            return results

        for filename in os.listdir(project_path):
            if not filename.endswith(".json"):
                continue

            path = os.path.join(project_path, filename)
            try:
                with open(path, "r") as f:
                    data = json.load(f)

                content_str = json.dumps(data.get("content", "")).lower()
                metadata = data.get("metadata", {})

                if query_lower in content_str:
                    if filters:
                        match = all(metadata.get(k) == v for k, v in filters.items())
                        if not match:
                            continue

                    results.append(
                        {
                            "key": data.get("key"),
                            "content": data.get("content"),
                            "metadata": metadata,
                            "relevance": "keyword_match",
                        }
                    )

                    if len(results) >= limit:
                        break
            except Exception as e:
                logger.warning(f"Failed to read {filename}: {e}")

        return results

    def delete(self, project_id: str, key: str) -> bool:
        try:
            path = self._key_path(project_id, key)
            if os.path.exists(path):
                os.remove(path)
                return True
            return False
        except Exception as e:
            logger.error(f"Failed to delete {key}: {e}")
            return False

    def list_keys(self, project_id: str, prefix: str = None) -> List[str]:
        keys = []
        project_path = self._project_path(project_id)

        if not os.path.exists(project_path):
            return keys

        for filename in os.listdir(project_path):
            if filename.endswith(".json"):
                key = filename[:-5].replace("__", "/")
                if prefix is None or key.startswith(prefix):
                    keys.append(key)

        return keys


class SupabaseBackend(MemoryBackend):
    """
    Supabase-based memory backend.

    Stores memories in a `project_memory` table.
    Good for distributed access, but no semantic search yet.

    Table schema needed:
    CREATE TABLE project_memory (
        id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
        project_id UUID NOT NULL,
        key TEXT NOT NULL,
        content JSONB,
        metadata JSONB,
        embedding VECTOR(1536),  -- for future semantic search
        stored_at TIMESTAMPTZ DEFAULT NOW(),
        UNIQUE(project_id, key)
    );
    """

    def __init__(self, supabase_client):
        self.client = supabase_client
        self.table = "project_memory"

    def store(
        self, project_id: str, key: str, content: Any, metadata: Dict = None
    ) -> bool:
        try:
            self.client.table(self.table).upsert(
                {
                    "project_id": project_id,
                    "key": key,
                    "content": content,
                    "metadata": metadata or {},
                }
            ).execute()
            return True
        except Exception as e:
            logger.error(f"Supabase store failed: {e}")
            return False

    def retrieve(self, project_id: str, key: str) -> Optional[Any]:
        try:
            result = (
                self.client.table(self.table)
                .select("content")
                .eq("project_id", project_id)
                .eq("key", key)
                .execute()
            )
            if result.data:
                return result.data[0].get("content")
            return None
        except Exception as e:
            logger.error(f"Supabase retrieve failed: {e}")
            return None

    def search(
        self, project_id: str, query: str, limit: int = 10, filters: Dict = None
    ) -> List[Dict]:
        try:
            q = self.client.table(self.table).select("*").eq("project_id", project_id)

            if filters:
                for k, v in filters.items():
                    q = q.contains("metadata", {k: v})

            result = q.limit(limit).execute()

            results = []
            for row in result.data or []:
                content_str = json.dumps(row.get("content", "")).lower()
                if query.lower() in content_str:
                    results.append(
                        {
                            "key": row.get("key"),
                            "content": row.get("content"),
                            "metadata": row.get("metadata"),
                            "relevance": "keyword_match",
                        }
                    )

            return results
        except Exception as e:
            logger.error(f"Supabase search failed: {e}")
            return []

    def delete(self, project_id: str, key: str) -> bool:
        try:
            self.client.table(self.table).delete().eq("project_id", project_id).eq(
                "key", key
            ).execute()
            return True
        except Exception as e:
            logger.error(f"Supabase delete failed: {e}")
            return False

    def list_keys(self, project_id: str, prefix: str = None) -> List[str]:
        try:
            q = self.client.table(self.table).select("key").eq("project_id", project_id)
            result = q.execute()

            keys = [row["key"] for row in result.data or []]
            if prefix:
                keys = [k for k in keys if k.startswith(prefix)]

            return keys
        except Exception as e:
            logger.error(f"Supabase list_keys failed: {e}")
            return []


class Memory:
    """
    VibePilot Memory System.

    Usage:
        memory = Memory()  # Uses FileBackend by default
        memory = Memory(backend="supabase")  # Uses SupabaseBackend

        # For future: memory = Memory(backend="vector", embedding_model="...")
    """

    def __init__(self, backend: str = "file", **kwargs):
        self.backend_name = backend

        if backend == "file":
            self._backend = FileBackend(**kwargs)
        elif backend == "supabase":
            from supabase import create_client
            import os

            url = os.getenv("SUPABASE_URL")
            key = os.getenv("SUPABASE_KEY")
            client = create_client(url, key)
            self._backend = SupabaseBackend(client)
        else:
            raise ValueError(f"Unknown backend: {backend}. Options: file, supabase")

        logger.info(f"Memory initialized with {backend} backend")

    def store(
        self, project_id: str, key: str, content: Any, metadata: Dict = None
    ) -> bool:
        """Store content. Key can be hierarchical: 'decisions/auth/001'"""
        return self._backend.store(project_id, key, content, metadata)

    def retrieve(self, project_id: str, key: str) -> Optional[Any]:
        """Retrieve content by key."""
        return self._backend.retrieve(project_id, key)

    def search(
        self, project_id: str, query: str, limit: int = 10, filters: Dict = None
    ) -> List[Dict]:
        """Search for relevant content."""
        return self._backend.search(project_id, query, limit, filters)

    def delete(self, project_id: str, key: str) -> bool:
        """Delete content by key."""
        return self._backend.delete(project_id, key)

    def list_keys(self, project_id: str, prefix: str = None) -> List[str]:
        """List all keys, optionally filtered by prefix."""
        return self._backend.list_keys(project_id, prefix)

    def store_decision(self, project_id: str, decision_id: str, decision: Dict):
        """Convenience: Store a decision with standard metadata."""
        return self.store(
            project_id,
            f"decisions/{decision_id}",
            decision,
            {"type": "decision", "stored_at": datetime.utcnow().isoformat()},
        )

    def store_research(self, project_id: str, topic: str, findings: Dict):
        """Convenience: Store research findings."""
        return self.store(
            project_id,
            f"research/{topic}",
            findings,
            {"type": "research", "stored_at": datetime.utcnow().isoformat()},
        )

    def store_pattern(self, project_id: str, pattern_name: str, pattern: Dict):
        """Convenience: Store a reusable pattern."""
        return self.store(
            project_id,
            f"patterns/{pattern_name}",
            pattern,
            {"type": "pattern", "stored_at": datetime.utcnow().isoformat()},
        )

    def get_recent_decisions(self, project_id: str, limit: int = 20) -> List[Dict]:
        """Get recent decisions for a project."""
        keys = self.list_keys(project_id, prefix="decisions/")
        decisions = []
        for key in keys[-limit:]:
            content = self.retrieve(project_id, key)
            if content:
                decisions.append({"key": key, "content": content})
        return decisions


if __name__ == "__main__":
    print("=== VibePilot Memory Interface ===\n")

    memory = Memory(backend="file")
    project = "test-project"

    print("Storing test data...")
    memory.store_decision(
        project,
        "DEC-001",
        {"title": "Use Python for backend", "reason": "Existing codebase, AI-friendly"},
    )

    memory.store_research(
        project,
        "vector-dbs",
        {
            "findings": [
                "Pinecone managed",
                "Weaviate self-hosted",
                "pgvector Postgres",
            ],
            "recommendation": "pgvector for Supabase compatibility",
        },
    )

    print("\nListing keys:")
    for key in memory.list_keys(project):
        print(f"  - {key}")

    print("\nSearching for 'python':")
    results = memory.search(project, "python")
    for r in results:
        print(f"  - {r['key']}: {r['content']}")

    print("\n✅ Memory interface ready for pluggable backends")
