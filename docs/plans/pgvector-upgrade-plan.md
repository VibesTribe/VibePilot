# PLAN: pgvector Critical Upgrade for X220 HDD Performance

## Overview
This plan outlines the steps to upgrade the pgvector extension in PostgreSQL from v0.6.0 to v0.8.2. The primary goal is to address performance bottlenecks on the ThinkPad X220, specifically those related to its spinning HDD and limited RAM. The upgrade will enable features like `halfvec` storage, `binary_quantize` for pre-filtering, and improved index scan performance, leading to significant storage reduction and faster query times for the `kb_context_pack` table. This is a system infrastructure improvement with a zero-cost, zero-lock-in, and reversible implementation.

## Tasks

### T001: Update pgvector Extension to v0.8.2
**Confidence:** 0.99
**Category:** coding
**Dependencies:** none
**Target Files:** PostgreSQL database schema

#### Prompt Packet
```markdown
# TASK: T001 - Update pgvector Extension to v0.8.2

## Context
The current pgvector extension version (0.6.0) lacks critical features for optimizing performance on spinning HDDs, specifically `halfvec` storage and `binary_quantize` pre-filtering. This task aims to upgrade the extension to v0.8.2 to unlock these optimizations.

## What to Build
1. Connect to the PostgreSQL 16.14 instance.
2. Execute the command `ALTER EXTENSION vector UPDATE;` to upgrade the pgvector extension to the latest installed version (which should be 0.8.2 or higher, as specified by the plan).
3. Verify the extension version using `SELECT extversion FROM pg_extension WHERE extname = 'vector';` to ensure it is reporting v0.8.2 or greater.

**Do NOT:**
- Modify any PostgreSQL configuration files.
- Restart PostgreSQL if not explicitly required by the extension upgrade process (which `ALTER EXTENSION` typically handles gracefully).

## Files
- `postgresql_upgrade.sql` - A script containing the SQL commands to perform the upgrade and verification.
```

#### Expected Output
```json
{
  "task_id": "T001",
  "files_created": [
    "postgresql_upgrade.sql"
  ],
  "tests_written": [],
  "db_operations": [
    "ALTER EXTENSION vector UPDATE;",
    "SELECT extversion FROM pg_extension WHERE extname = 'vector';"
  ],
  "verification_steps": [
    "Confirm successful execution of ALTER EXTENSION command.",
    "Confirm SELECT query returns '0.8.2' or higher for vector extension version."
  ]
}
```

### T002: Migrate Embedding Columns to halfvec(768)
**Confidence:** 0.98
**Category:** coding
**Dependencies:** T001
**Target Files:** PostgreSQL database schema

#### Prompt Packet
```markdown
# TASK: T002 - Migrate Embedding Columns to halfvec(768)

## Context
The current `vector(768)` columns in the `kb_context_pack` table consume approximately twice the storage compared to the `halfvec(768)` type. This migration is critical for reducing storage footprint and improving performance on the X220's limited hardware.

## What to Build
1. Connect to the PostgreSQL 16.14 instance.
2. For the `kb_context_pack` table, alter the `embedding` column (or any other columns storing vector embeddings) to use the `halfvec(768)` type.
3. Ensure the migration process is as non-disruptive as possible, aiming for concurrent writes if PostgreSQL and pgvector version support it.
4. Measure the storage usage before and after the migration using `pg_total_relation_size('kb_context_pack');` to confirm at least a 45% reduction.
5. Perform spot checks on a diverse set of code search queries to ensure search relevance has not dropped. This can be a qualitative check initially, confirming expected results are returned.

**Do NOT:**
- Alter column names unless absolutely necessary for compatibility.
- Delete original data before confirming successful conversion and data integrity.

## Files
- `migrate_embedding_to_halfvec.sql` - A script containing the SQL commands for the column type migration and verification.
```

#### Expected Output
```json
{
  "task_id": "T002",
  "files_created": [
    "migrate_embedding_to_halfvec.sql"
  ],
  "tests_written": [],
  "db_operations": [
    "ALTER TABLE kb_context_pack ALTER COLUMN embedding TYPE halfvec(768);",
    "pg_total_relation_size('kb_context_pack');"
  ],
  "verification_steps": [
    "Confirm successful execution of ALTER TABLE command.",
    "Verify storage reduction is >= 45% via pg_total_relation_size.",
    "Perform spot-check searches to confirm relevance is maintained."
  ]
}
```

### T003: Add binary_quantize Column for Fast Pre-filtering
**Confidence:** 0.98
**Category:** coding
**Dependencies:** T001
**Target Files:** PostgreSQL database schema

#### Prompt Packet
```markdown
# TASK: T003 - Add binary_quantize Column for Fast Pre-filtering

## Context
The `binary_quantize` feature in pgvector v0.8.0+ allows for a faster, preliminary filtering step using Hamming distance on binary representations of embeddings. This is crucial for reducing disk I/O on the spinning HDD by allowing queries to quickly eliminate large portions of the index before performing more expensive distance calculations.

## What to Build
1. Connect to the PostgreSQL 16.14 instance.
2. Add a new column, e.g., `embedding_binary`, of type `bytea` to the `kb_context_pack` table.
3. Populate this new column by applying the `binary_quantize()` function to the existing `embedding` (now `halfvec`) column. This should ideally be done in a way that minimizes locking and allows concurrent reads.
4. Verify the column is created and populated correctly.
5. Update relevant similarity search queries to utilize this new column for initial filtering. This will involve modifying queries to use Hamming distance first, followed by L2/cosine distance on the remaining candidates.
6. Use `EXPLAIN ANALYZE` on sample filtered search queries to confirm the query plan now involves the `embedding_binary` column and shows reduced disk I/O compared to a plan without it.

**Do NOT:**
- Remove the original `embedding` (halfvec) column until the new column and its usage are fully validated.
- Assume a specific query structure; the task is to *enable* the feature and verify its *usage* in relevant queries.

## Files
- `add_binary_quantize_column.sql` - A script containing the SQL commands for adding the column, populating it, and potentially an example query demonstrating its usage.
```

#### Expected Output
```json
{
  "task_id": "T003",
  "files_created": [
    "add_binary_quantize_column.sql"
  ],
  "tests_written": [],
  "db_operations": [
    "ALTER TABLE kb_context_pack ADD COLUMN embedding_binary bytea;",
    "UPDATE kb_context_pack SET embedding_binary = binary_quantize(embedding);",
    "EXPLAIN ANALYZE ..."
  ],
  "verification_steps": [
    "Confirm `embedding_binary` column is added.",
    "Verify column is populated with bytea data.",
    "Verify sample queries use `embedding_binary` for filtering via EXPLAIN ANALYZE.",
    "Confirm reduction in disk I/O for filtered queries."
  ]
}
```

### T004: Rebuild HNSW Indexes
**Confidence:** 0.98
**Category:** coding
**Dependencies:** T001, T002, T003
**Target Files:** PostgreSQL database schema

#### Prompt Packet
```markdown
# TASK: T004 - Rebuild HNSW Indexes

## Context
pgvector versions 0.8.0 and later include performance improvements for HNSW index building and scanning. Rebuilding these indexes is necessary to leverage these enhancements, particularly for filtered vector searches on the `kb_context_pack` table, which is crucial for HDD performance.

## What to Build
1. Connect to the PostgreSQL 16.14 instance.
2. Identify all HNSW indexes on the `kb_context_pack` table that were previously used for vector similarity searches.
3. Rebuild these HNSW indexes. Utilize PostgreSQL's `CONCURRENTLY` option where possible to minimize downtime and allow for concurrent reads/writes during the rebuild process.
4. Measure the index build time for each index and compare it to a baseline measurement taken before the upgrade (or a simulated baseline if not available) to confirm improvement.
5. Run benchmark queries (e.g., similarity searches with filters) and use `EXPLAIN ANALYZE` to confirm query performance has improved by at least 25% compared to the pre-upgrade baseline.

**Do NOT:**
- Drop existing indexes without ensuring new ones are successfully created.
- Ignore index build times; they are a key performance indicator.

## Files
- `rebuild_hnsw_indexes.sql` - A script containing the SQL commands for rebuilding the indexes and potentially benchmark queries.
```

#### Expected Output
```json
{
  "task_id": "T004",
  "files_created": [
    "rebuild_hnsw_indexes.sql"
  ],
  "tests_written": [],
  "db_operations": [
    "DROP INDEX IF EXISTS ...;",
    "CREATE INDEX ... ON kb_context_pack USING ivfflat (...) WITH (lists = 100);",
    "CREATE INDEX CONCURRENTLY ... ON kb_context_pack USING hnsw (...) WITH (m=16, ef_construction=64);",
    "EXPLAIN ANALYZE ..."
  ],
  "verification_steps": [
    "Confirm indexes are rebuilt and using appropriate HNSW parameters.",
    "Measure and confirm index build time improvement.",
    "Verify query performance improvement >= 25% for filtered vector searches."
  ]
}
```

### T005: Update Go pgx Dependency to v5.10.0+
**Confidence:** 0.99
**Category:** coding
**Dependencies:** T001
**Target Files:** `go.mod`, `go.sum`, Hermes Agent source code

#### Prompt Packet
```markdown
# TASK: T005 - Update Go pgx Dependency to v5.10.0+

## Context
The Hermes Agent uses the `jackc/pgx` Go driver to interact with PostgreSQL. Versions prior to v5.10.0 do not fully support the new `halfvec` and `bytea` (for `binary_quantize`) types introduced in pgvector v0.8.0+. Updating to v5.10.0+ is critical for the Hermes Agent to correctly handle and utilize the new data types and features enabled by the pgvector upgrade.

## What to Build
1. Navigate to the Hermes Agent's Go module directory.
2. Update the `go.mod` file to specify `github.com/jackc/pgx/v5` with a version of v5.10.0 or later.
3. Run `go mod tidy` to resolve dependencies and update `go.sum`.
4. Compile the Hermes Agent. Ensure there are no compilation errors related to pgx type handling.
5. Run existing API endpoints that interact with vector data (e.g., embeddings search, data ingestion) and verify they function correctly without runtime panics or errors related to vector types.

**Do NOT:**
- Pin to a specific minor version unless absolutely necessary; allow `go mod tidy` to find the latest compatible patch/minor version.
- Introduce any business logic changes; focus solely on the dependency update and compilation/runtime verification.

## Files
- `go.mod` - Updated file specifying the new pgx version.
- `go.sum` - Updated file reflecting the new dependency.
- Hermes Agent source code (implicitly modified by `go mod tidy` and potential type adjustments).
```

#### Expected Output
```json
{
  "task_id": "T005",
  "files_created": [
    "go.mod",
    "go.sum"
  ],
  "tests_written": [],
  "verification_steps": [
    "Confirm Hermes Agent compiles successfully.",
    "Verify no runtime errors related to pgx vector types.",
    "Confirm API endpoints using vector operations return successfully."
  ],
  "dependency_updates": [
    {
      "package": "github.com/jackc/pgx/v5",
      "from_version": "<previous_version>",
      "to_version": ">=v5.10.0"
    }
  ]
}
```
