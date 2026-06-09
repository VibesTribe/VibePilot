# PLAN: pgvector Critical Upgrade for X220 HDD Performance

## Overview
Upgrade pgvector from v0.6.0 to v0.8.2 to enable halfvec storage, iterative index scans, and binary quantize - critical for VibePilot's kb_context_pack performance on the X220's spinning HDD within zero-cost constraints.

## Tasks

### T001: Update pgvector Extension
**Confidence:** 0.99
**Category:** database
**Dependencies:** none
**Target Files:** None

#### Prompt Packet
```
# TASK: T001 - Update pgvector Extension

## Context
This task upgrades the pgvector extension from v0.6.0 to v0.8.2 to access new features like halfvec storage, binary_quantize, and improved index scans that are essential for HDD performance on the X220.

## What to Build
Execute the necessary SQL command to update the pgvector extension to version 0.8.2 in the PostgreSQL database.

## Files
- None (this is a direct database operation)
```

#### Expected Output
```json
{
  "files_created": [],
  "tests_required": [],
  "database_change": "pgvector extension updated to version 0.8.2"
}
```

### T002: Convert Embedding Columns to halfvec
**Confidence:** 0.96
**Category:** database
**Dependencies:** T001
**Target Files:** None

#### Prompt Packet
```
# TASK: T002 - Convert Embedding Columns to halfvec

## Context
After upgrading pgvector to v0.8.2, we need to convert the existing vector(768) columns in the kb_context_pack table to halfvec(768) to achieve ~50% storage reduction for embeddings, which is critical for the X220's limited RAM and spinning HDD.

## What to Build
Convert the embedding column in the kb_context_pack table from vector(768) to halfvec(768) type. This must handle any indexes on the column appropriately:
1. Identify indexes on kb_context_pack.embedding column
2. Drop those indexes (if any exist)
3. Alter the column type to halfvec(768) USING embedding::halfvec
4. Recreate the indexes (if any were dropped)

## Files
- None (this is a direct database operation)
```

#### Expected Output
```json
{
  "files_created": [],
  "tests_required": [],
  "database_change": "kb_context_pack.embedding column converted to halfvec(768) type"
}
```

### T003: Update Go pgx Dependency
**Confidence:** 0.99
**Category:** dependency
**Dependencies:** T001
**Target Files:** go.mod

#### Prompt Packet
```
# TASK: T003 - Update Go pgx Dependency

## Context
The Hermes agent uses the Go pgx driver to interact with PostgreSQL. To support the new halfvec and binary_quantize types introduced in pgvector v0.8.0+, we need to update the pgx dependency to v5.10.0 or later, as earlier versions lack full support for these types.

## What to Build
Update the go.mod file to require github.com/jackc/pgx/v5.10.0 or later version, then run go mod tidy to update dependencies.

## Files
- go.mod - The Go module file to update
```

#### Expected Output
```json
{
  "files_created": [],
  "tests_required": [],
  "files_modified": ["go.mod"],
  "dependency_change": "Updated pgx dependency to v5.10.0+"
}
```

### T004: Add Binary Quantize Column
**Confidence:** 0.99
**Category:** database
**Dependencies:** T002
**Target Files:** None

#### Prompt Packet
```
# TASK: T004 - Add Binary Quantize Column

## Context
To enable fast pre-filtering of vector searches on the X220's spinning HDD, we need to add a binary_quantized_embedding column that stores the binary quantization of embeddings using hamming distance for first-pass search, reducing disk I/O.

## What to Build
Add a new column named binary_quantized_embedding of type bytea to the kb_context_pack table. This column will store the result of binary_quantize(embedding) for each row.

## Files
- None (this is a direct database operation)
```

#### Expected Output
```json
{
  "files_created": [],
  "tests_required": [],
  "database_change": "Added binary_quantized_embedding column (bytea) to kb_context_pack table"
}
```

### T005: Rebuild HNSW Indexes
**Confidence:** 0.95
**Category:** database
**Dependencies:** T004
**Target Files:** None

#### Prompt Packet
```
# TASK: T005 - Rebuild HNSW Indexes

## Context
After upgrading pgvector and converting column types, rebuilding HNSW indexes allows us to leverage v0.8.0+ improvements in insert/scan performance, which is particularly beneficial for the X220's spinning HDD by reducing query latency.

## What to Build
Rebuild all HNSW indexes on embedding columns in the kb_context_pack table to leverage performance improvements in pgvector v0.8.0+. This should be done using the REINDEX command or equivalent to rebuild the indexes online where possible.

## Files
- None (this is a direct database operation)
```

#### Expected Output
```json
{
  "files_created": [],
  "tests_required": [],
  "database_change": "Rebuilt all HNSW indexes on kb_context_pack embedding columns"
}
```

### T006: Verify Upgrade Success
**Confidence:** 0.95
**Category:** testing
**Dependencies:** T005
**Target Files:** None

#### Prompt Packet
```
# TASK: T006 - Verify Upgrade Success

## Context
After performing the pgvector upgrade and related changes, we need to verify that the upgrade was successful and that the system is functioning correctly with the new features.

## What to Build
Execute verification SQL queries to confirm:
1. pgvector version is 0.8.2 or higher
2. The embedding column in kb_context_pack is now halfvec type
3. The binary_quantized_embedding column exists and is bytea type
4. A simple vector search query works correctly
5. Basic Hermes agent database operations would succeed

## Files
- None (this is a verification task)
```

#### Expected Output
```json
{
  "files_created": [],
  "tests_required": [],
  "verification_results": {
    "pgvector_version": "0.8.2+",
    "embedding_column_type": "halfvec",
    "binary_column_exists": true,
    "sample_query_success": true
  }
}
```

### T007: Verify Hermes Agent Compatibility
**Confidence:** 0.96
**Category:** testing
**Dependencies:** T003, T006
**Target Files:** None

#### Prompt Packet
```
# TASK: T007 - Verify Hermes Agent Compatibility

## Context
After updating the Go pgx dependency, we need to verify that the Hermes agent can still build and run correctly with the new pgx version, ensuring it properly handles the halfvec and binary_quantize types from the database.

## What to Build
1. Run 'go build' to verify the Hermes agent compiles successfully with the updated pgx dependency
2. Run unit tests related to database/vector operations (if any exist) to verify type handling works
3. If no specific tests exist, verify that the code imports and uses pgx types correctly for vector operations

## Files
- None (this is a build/test task)
```

#### Expected Output
```json
{
  "files_created": [],
  "tests_required": [],
  "build_success": true,
  "test_success": true,
  "compatibility_verified": true
}
```

### T008: Create Documentation
**Confidence:** 0.98
**Category:** documentation
**Dependencies:** T007
**Target Files:** docs/upgrades/pgvector-v0.8.2-upgrade.md

#### Prompt Packet
```
# TASK: T008 - Create Upgrade Documentation

## Context
Documenting the upgrade procedure and rollback steps is essential for maintaining system stability and allowing future administrators to understand and potentially reverse the changes if needed, in line with VibePilot's reversibility requirement.

## What to Build
Create a comprehensive markdown document that includes:
1. Overview of the upgrade and its benefits
2. Prerequisites and system state before upgrade
3. Step-by-step upgrade procedure with exact commands
4. Verification steps to confirm success
5. Rollback procedure to revert changes if needed
6. Performance impact analysis
7. Timeline and maintenance window recommendations

## Files
- docs/upgrades/pgvector-v0.8.2-upgrade.md - The documentation file to create
```

#### Expected Output
```json
{
  "files_created": ["docs/upgrades/pgvector-v0.8.2-upgrade.md"],
  "tests_required": [],
  "documentation_created": true
}
```
  ],
  "total_tasks": 8,
  "status": "review