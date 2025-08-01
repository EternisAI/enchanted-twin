-- Memory Facts Queries

-- name: GetMemoryFact :one
SELECT * FROM memory_facts
WHERE id = $1;

-- name: CreateMemoryFact :one
INSERT INTO memory_facts (
    id, content, content_vector, timestamp, source, tags, document_references,
    metadata_json, fact_category, fact_subject, fact_attribute, fact_value,
    fact_temporal_context, fact_sensitivity, fact_importance, fact_file_path
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16
) RETURNING *;

-- name: UpdateMemoryFact :one
UPDATE memory_facts SET
    content = $2,
    content_vector = $3,
    timestamp = $4,
    source = $5,
    tags = $6,
    document_references = $7,
    metadata_json = $8,
    fact_category = $9,
    fact_subject = $10,
    fact_attribute = $11,
    fact_value = $12,
    fact_temporal_context = $13,
    fact_sensitivity = $14,
    fact_importance = $15,
    fact_file_path = $16
WHERE id = $1
RETURNING *;

-- name: DeleteMemoryFact :exec
DELETE FROM memory_facts WHERE id = $1;

-- name: QueryMemoryFactsByVector :many
SELECT *, content_vector <=> @content_vector AS distance
FROM memory_facts
WHERE (@source_filter::text = '' OR source = @source_filter)
  AND (@category_filter::text = '' OR fact_category = @category_filter)
  AND (@subject_filter::text = '' OR fact_subject ILIKE '%' || @subject_filter || '%')
  AND (@importance_exact::int = 0 OR fact_importance = @importance_exact)
  AND (@importance_min::int = 0 OR fact_importance >= @importance_min)
  AND (@importance_max::int = 0 OR fact_importance <= @importance_max)
  AND (@timestamp_start::timestamptz IS NULL OR timestamp >= @timestamp_start)
  AND (@timestamp_end::timestamptz IS NULL OR timestamp <= @timestamp_end)
  AND (@file_path_filter::text = '' OR fact_file_path = @file_path_filter)
  AND (@tags_filter::text[] IS NULL OR tags && @tags_filter)
  AND (@document_refs_filter::text[] IS NULL OR document_references && @document_refs_filter)
  AND (@max_distance::float = 0 OR content_vector <=> @content_vector <= @max_distance)
ORDER BY content_vector <=> @content_vector
LIMIT @limit_count;

-- name: QueryMemoryFactsFilterOnly :many
SELECT *
FROM memory_facts
WHERE ($1::text IS NULL OR source = $1)
  AND ($2::text IS NULL OR fact_category = $2)
  AND ($3::text IS NULL OR fact_subject ILIKE '%' || $3 || '%')
  AND ($4::int IS NULL OR fact_importance = $4)
  AND ($5::int IS NULL OR fact_importance >= $5)
  AND ($6::int IS NULL OR fact_importance <= $6)
  AND ($7::timestamptz IS NULL OR timestamp >= $7)
  AND ($8::timestamptz IS NULL OR timestamp <= $8)
  AND ($9::text IS NULL OR fact_file_path = $9)
  AND ($10::text[] IS NULL OR tags && $10)
  AND ($11::text[] IS NULL OR document_references && $11)
ORDER BY created_at DESC
LIMIT $12;

-- name: GetMemoryFactsByIDs :many
SELECT * FROM memory_facts
WHERE id = ANY($1::uuid[])
LIMIT 1000;

-- Source Documents Queries

-- name: GetSourceDocument :one
SELECT * FROM source_documents
WHERE id = $1;

-- name: CreateSourceDocument :one
INSERT INTO source_documents (
    id, content, content_hash, document_type, original_id, metadata_json
) VALUES (
    $1, $2, $3, $4, $5, $6
) RETURNING *;

-- name: GetSourceDocumentByHash :one
SELECT * FROM source_documents
WHERE content_hash = $1;

-- name: GetSourceDocumentsBatch :many
SELECT * FROM source_documents
WHERE id = ANY($1::uuid[])
LIMIT 1000;

-- Document Chunks Queries

-- name: GetDocumentChunk :one
SELECT * FROM document_chunks
WHERE id = $1;

-- name: CreateDocumentChunk :one
INSERT INTO document_chunks (
    id, content, content_vector, chunk_index, original_document_id,
    source, file_path, tags, metadata_json
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
) RETURNING *;

-- name: QueryDocumentChunksByVector :many
SELECT *, content_vector <=> $1 AS distance
FROM document_chunks
WHERE ($2::text = '' OR source = $2)
  AND ($3::text = '' OR file_path = $3)
  AND ($4::text[] IS NULL OR tags && $4)
  AND ($6::float = 0 OR content_vector <=> $1 <= $6)
ORDER BY content_vector <=> $1
LIMIT $5;

-- name: GetDocumentChunksByOriginalDocument :many
SELECT * FROM document_chunks
WHERE original_document_id = $1
ORDER BY chunk_index;