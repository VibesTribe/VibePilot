-- pgvector 0.6.0 -> 0.8.2 upgrade: halfvec, bitvec, binary quantization
-- Run after: ALTER EXTENSION vector UPDATE; (requires superuser)

-- 1. Add binary quantize columns (bit vectors for ultra-fast approximate search)
ALTER TABLE kb_claims ADD COLUMN IF NOT EXISTS embedding_bq bit(2000);
ALTER TABLE kb_code_symbols ADD COLUMN IF NOT EXISTS embedding_bq bit(2000);
ALTER TABLE kb_doc_sections ADD COLUMN IF NOT EXISTS embedding_bq bit(2000);
ALTER TABLE kb_knowledge_items ADD COLUMN IF NOT EXISTS embedding_bq bit(2000);

UPDATE kb_claims SET embedding_bq = binary_quantize(embedding) WHERE embedding IS NOT NULL;
UPDATE kb_code_symbols SET embedding_bq = binary_quantize(embedding) WHERE embedding IS NOT NULL;
UPDATE kb_doc_sections SET embedding_bq = binary_quantize(embedding) WHERE embedding IS NOT NULL;
UPDATE kb_knowledge_items SET embedding_bq = binary_quantize(embedding) WHERE embedding IS NOT NULL;

-- 2. Convert float vectors to half-precision (2 bytes/dim instead of 4, ~50% storage savings)
-- Storage: float=8004 bytes/row, halfvec=4004 bytes/row, bq=258 bytes/row
ALTER TABLE kb_claims ADD COLUMN IF NOT EXISTS embedding_half halfvec(2000);
ALTER TABLE kb_code_symbols ADD COLUMN IF NOT EXISTS embedding_half halfvec(2000);
ALTER TABLE kb_doc_sections ADD COLUMN IF NOT EXISTS embedding_half halfvec(2000);
ALTER TABLE kb_knowledge_items ADD COLUMN IF NOT EXISTS embedding_half halfvec(2000);

UPDATE kb_claims SET embedding_half = embedding::halfvec WHERE embedding IS NOT NULL;
UPDATE kb_code_symbols SET embedding_half = embedding::halfvec WHERE embedding IS NOT NULL;
UPDATE kb_doc_sections SET embedding_half = embedding::halfvec WHERE embedding IS NOT NULL;
UPDATE kb_knowledge_items SET embedding_half = embedding::halfvec WHERE embedding IS NOT NULL;

-- 3. Swap columns: drop float, rename halfvec to embedding
ALTER TABLE kb_claims DROP COLUMN embedding;
ALTER TABLE kb_claims RENAME COLUMN embedding_half TO embedding;

ALTER TABLE kb_code_symbols DROP COLUMN embedding;
ALTER TABLE kb_code_symbols RENAME COLUMN embedding_half TO embedding;

ALTER TABLE kb_doc_sections DROP COLUMN embedding;
ALTER TABLE kb_doc_sections RENAME COLUMN embedding_half TO embedding;

ALTER TABLE kb_knowledge_items DROP COLUMN embedding;
ALTER TABLE kb_knowledge_items RENAME COLUMN embedding_half TO embedding;

-- 4. Build HNSW indexes for fast similarity search
-- Primary: cosine distance on halfvec embeddings
CREATE INDEX CONCURRENTLY IF NOT EXISTS kb_claims_embedding_hnsw 
  ON kb_claims USING hnsw (embedding halfvec_cosine_ops) WITH (M = 16, ef_construction = 64);
CREATE INDEX CONCURRENTLY IF NOT EXISTS kb_code_symbols_embedding_hnsw 
  ON kb_code_symbols USING hnsw (embedding halfvec_cosine_ops) WITH (M = 16, ef_construction = 64);
CREATE INDEX CONCURRENTLY IF NOT EXISTS kb_doc_sections_embedding_hnsw 
  ON kb_doc_sections USING hnsw (embedding halfvec_cosine_ops) WITH (M = 16, ef_construction = 64);
CREATE INDEX CONCURRENTLY IF NOT EXISTS kb_knowledge_items_embedding_hnsw 
  ON kb_knowledge_items USING hnsw (embedding halfvec_cosine_ops) WITH (M = 16, ef_construction = 64);

-- Secondary: hamming distance on binary quantized (ultra-fast approximate)
CREATE INDEX CONCURRENTLY IF NOT EXISTS kb_claims_bq_hnsw 
  ON kb_claims USING hnsw (embedding_bq bit_hamming_ops) WITH (M = 16, ef_construction = 64);
CREATE INDEX CONCURRENTLY IF NOT EXISTS kb_code_symbols_bq_hnsw 
  ON kb_code_symbols USING hnsw (embedding_bq bit_hamming_ops) WITH (M = 16, ef_construction = 64);
CREATE INDEX CONCURRENTLY IF NOT EXISTS kb_doc_sections_bq_hnsw 
  ON kb_doc_sections USING hnsw (embedding_bq bit_hamming_ops) WITH (M = 16, ef_construction = 64);
CREATE INDEX CONCURRENTLY IF NOT EXISTS kb_knowledge_items_bq_hnsw 
  ON kb_knowledge_items USING hnsw (embedding_bq bit_hamming_ops) WITH (M = 16, ef_construction = 64);
