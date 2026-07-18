-- name: InitializeCatalog :exec
INSERT INTO catalog(singleton)
VALUES (1)
ON CONFLICT(singleton) DO NOTHING;

-- name: ValidateSchemaShape :one
WITH
  generation_shape AS (
    SELECT id, vector_format_version, model, dimension, protocol_epoch,
           chunker_epoch, vault_root, corpus_policy_fingerprint,
           policy_source_fingerprint, target_corpus_fingerprint,
           expected_chunks, top_k_p95_us
    FROM generations LIMIT 0
  ),
  catalog_shape AS (
    SELECT singleton, active_generation_id, previous_generation_id,
           staging_generation_id
    FROM catalog LIMIT 0
  ),
  note_shape AS (
    SELECT generation_id, rel_path, note_hash FROM notes LIMIT 0
  ),
  chunk_shape AS (
    SELECT generation_id, rel_path, ordinal, submitted_hash, vector
    FROM chunks LIMIT 0
  ),
  attempt_shape AS (
    SELECT generation_id, rel_path, ordinal, attempts,
           retry_not_before_unix_ms
    FROM attempts LIMIT 0
  )
SELECT (SELECT count(*) FROM generation_shape) +
       (SELECT count(*) FROM catalog_shape) +
       (SELECT count(*) FROM note_shape) +
       (SELECT count(*) FROM chunk_shape) +
       (SELECT count(*) FROM attempt_shape);

-- name: Catalog :one
SELECT active_generation_id, previous_generation_id, staging_generation_id
FROM catalog
WHERE singleton = 1;

-- name: LogicalForeignKeyViolationCount :one
WITH violations AS (
  SELECT 1
  FROM catalog AS role
  LEFT JOIN generations AS generation ON generation.id = role.active_generation_id
  WHERE role.active_generation_id IS NOT NULL AND generation.id IS NULL
  UNION ALL
  SELECT 1
  FROM catalog AS role
  LEFT JOIN generations AS generation ON generation.id = role.previous_generation_id
  WHERE role.previous_generation_id IS NOT NULL AND generation.id IS NULL
  UNION ALL
  SELECT 1
  FROM catalog AS role
  LEFT JOIN generations AS generation ON generation.id = role.staging_generation_id
  WHERE role.staging_generation_id IS NOT NULL AND generation.id IS NULL
  UNION ALL
  SELECT 1
  FROM notes AS note
  LEFT JOIN generations AS generation ON generation.id = note.generation_id
  WHERE generation.id IS NULL
  UNION ALL
  SELECT 1
  FROM chunks AS chunk
  LEFT JOIN generations AS generation ON generation.id = chunk.generation_id
  WHERE generation.id IS NULL
  UNION ALL
  SELECT 1
  FROM chunks AS chunk
  LEFT JOIN notes AS note
    ON note.generation_id = chunk.generation_id AND note.rel_path = chunk.rel_path
  WHERE note.generation_id IS NULL
  UNION ALL
  SELECT 1
  FROM attempts AS attempt
  LEFT JOIN generations AS generation ON generation.id = attempt.generation_id
  WHERE generation.id IS NULL
  UNION ALL
  SELECT 1
  FROM attempts AS attempt
  LEFT JOIN chunks AS chunk
    ON chunk.generation_id = attempt.generation_id
   AND chunk.rel_path = attempt.rel_path
   AND chunk.ordinal = attempt.ordinal
  WHERE chunk.generation_id IS NULL
)
SELECT count(*) FROM violations;

-- name: GenerationByID :one
SELECT id, vector_format_version, model, dimension, protocol_epoch, chunker_epoch,
       vault_root, corpus_policy_fingerprint, policy_source_fingerprint,
       target_corpus_fingerprint, expected_chunks, top_k_p95_us
FROM generations
WHERE id = ?;

-- name: CreateGeneration :one
INSERT INTO generations(
  vector_format_version, model, dimension, protocol_epoch, chunker_epoch, vault_root,
  corpus_policy_fingerprint, policy_source_fingerprint,
  target_corpus_fingerprint, expected_chunks, top_k_p95_us
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0)
RETURNING id;

-- name: SetStaging :execrows
UPDATE catalog
SET staging_generation_id = ?
WHERE singleton = 1;

-- name: ClearStaging :execrows
UPDATE catalog
SET staging_generation_id = NULL
WHERE singleton = 1 AND staging_generation_id = ?;

-- name: InsertGenerationNote :execrows
INSERT INTO notes(generation_id, rel_path, note_hash)
VALUES (?, ?, ?)
ON CONFLICT(generation_id, rel_path) DO NOTHING;

-- name: InsertGenerationChunkTarget :execrows
INSERT INTO chunks(generation_id, rel_path, ordinal, submitted_hash, vector)
VALUES (?, ?, ?, ?, NULL)
ON CONFLICT(generation_id, rel_path, ordinal) DO NOTHING;

-- name: ReuseGenerationChunkVectors :execrows
UPDATE chunks
SET vector = (
  SELECT source.vector
  FROM chunks AS source
  WHERE source.generation_id = sqlc.arg(source_generation_id)
    AND source.submitted_hash = chunks.submitted_hash
    AND source.vector IS NOT NULL
  ORDER BY source.rel_path, source.ordinal
  LIMIT 1
)
WHERE chunks.generation_id = sqlc.arg(target_generation_id)
  AND chunks.vector IS NULL
  AND EXISTS (
    SELECT 1
    FROM generations AS source_generation
    JOIN generations AS target_generation
      ON target_generation.vector_format_version = source_generation.vector_format_version
     AND target_generation.model = source_generation.model
     AND target_generation.dimension = source_generation.dimension
     AND target_generation.protocol_epoch = source_generation.protocol_epoch
     AND target_generation.chunker_epoch = source_generation.chunker_epoch
     AND target_generation.vault_root = source_generation.vault_root
     AND target_generation.corpus_policy_fingerprint = source_generation.corpus_policy_fingerprint
    WHERE source_generation.id = sqlc.arg(source_generation_id)
      AND target_generation.id = sqlc.arg(target_generation_id)
  )
  AND EXISTS (
    SELECT 1
    FROM chunks AS source
    WHERE source.generation_id = sqlc.arg(source_generation_id)
      AND source.submitted_hash = chunks.submitted_hash
      AND source.vector IS NOT NULL
  );

-- name: CompleteGenerationChunk :execrows
UPDATE chunks
SET vector = sqlc.arg(vector)
WHERE chunks.generation_id = sqlc.arg(target_generation_id)
  AND chunks.rel_path = sqlc.arg(target_rel_path)
  AND chunks.ordinal = sqlc.arg(target_ordinal)
  AND chunks.submitted_hash = sqlc.arg(target_submitted_hash)
  AND chunks.vector IS NULL
  AND EXISTS (
    SELECT 1
    FROM notes
    WHERE notes.generation_id = chunks.generation_id
      AND notes.rel_path = chunks.rel_path
      AND notes.note_hash = sqlc.arg(target_note_hash)
  );

-- name: GenerationChunkPage :many
SELECT chunk.generation_id, chunk.rel_path, note.note_hash, chunk.ordinal,
       chunk.submitted_hash, chunk.vector
FROM chunks AS chunk
JOIN notes AS note
  ON note.generation_id = chunk.generation_id AND note.rel_path = chunk.rel_path
WHERE chunk.generation_id = sqlc.arg(generation_id)
  AND chunk.vector IS NOT NULL
  AND (
    chunk.rel_path > sqlc.arg(after_rel_path) OR
    (chunk.rel_path = sqlc.arg(after_rel_path) AND chunk.ordinal > sqlc.arg(after_ordinal))
  )
ORDER BY chunk.rel_path, chunk.ordinal
LIMIT sqlc.arg(page_size);

-- name: PendingGenerationChunks :many
SELECT chunk.generation_id, chunk.rel_path, note.note_hash, chunk.ordinal,
       chunk.submitted_hash
FROM chunks AS chunk
JOIN notes AS note
  ON note.generation_id = chunk.generation_id AND note.rel_path = chunk.rel_path
WHERE chunk.generation_id = ?
  AND chunk.vector IS NULL
ORDER BY chunk.rel_path, chunk.ordinal;

-- name: GenerationTargets :many
SELECT chunk.generation_id, chunk.rel_path, note.note_hash, chunk.ordinal,
       chunk.submitted_hash, chunk.vector IS NULL AS pending
FROM chunks AS chunk
JOIN notes AS note
  ON note.generation_id = chunk.generation_id AND note.rel_path = chunk.rel_path
WHERE chunk.generation_id = ?
ORDER BY chunk.rel_path, chunk.ordinal;

-- name: GenerationNoteCount :one
SELECT count(*)
FROM notes
WHERE generation_id = ?;

-- name: GenerationPendingCount :one
SELECT count(*)
FROM chunks
WHERE generation_id = ? AND vector IS NULL;

-- name: AttemptByKey :one
SELECT generation_id, rel_path, ordinal, attempts, retry_not_before_unix_ms
FROM attempts
WHERE generation_id = ? AND rel_path = ? AND ordinal = ?;

-- name: InsertAttempt :execrows
INSERT INTO attempts(
  generation_id, rel_path, ordinal, attempts, retry_not_before_unix_ms
) SELECT ?, ?, ?, 1, NULL
WHERE EXISTS (
  SELECT 1
  FROM chunks
  WHERE chunks.generation_id = sqlc.arg(generation_id)
    AND chunks.rel_path = sqlc.arg(rel_path)
    AND chunks.ordinal = sqlc.arg(ordinal)
    AND chunks.vector IS NULL
);

-- name: IncrementAttempt :execrows
UPDATE attempts
SET attempts = attempts + 1, retry_not_before_unix_ms = NULL
WHERE generation_id = ? AND rel_path = ? AND ordinal = ? AND attempts < 5;

-- name: SetRetryNotBefore :execrows
UPDATE attempts
SET retry_not_before_unix_ms = ?
WHERE generation_id = ? AND rel_path = ? AND ordinal = ?;

-- name: DeleteAttempt :execrows
DELETE FROM attempts
WHERE generation_id = ? AND rel_path = ? AND ordinal = ?;

-- name: GenerationAttemptCount :one
SELECT count(*)
FROM attempts
WHERE generation_id = ?;

-- name: AttemptsByGeneration :many
SELECT attempt.generation_id, attempt.rel_path, attempt.ordinal,
       attempt.attempts, attempt.retry_not_before_unix_ms,
       chunk.vector IS NULL AS pending
FROM attempts AS attempt
JOIN chunks AS chunk
  ON chunk.generation_id = attempt.generation_id
 AND chunk.rel_path = attempt.rel_path
 AND chunk.ordinal = attempt.ordinal
WHERE attempt.generation_id = ?
ORDER BY attempt.rel_path, attempt.ordinal;

-- name: SetGenerationTopKP95 :execrows
UPDATE generations
SET top_k_p95_us = ?
WHERE id = ?;

-- name: ActivateStaging :execrows
UPDATE catalog
SET previous_generation_id = active_generation_id,
    active_generation_id = staging_generation_id,
    staging_generation_id = NULL
WHERE singleton = 1 AND staging_generation_id = ?;

-- name: DeleteGeneration :execrows
DELETE FROM generations
WHERE id = ?;

-- name: DeleteUnreferencedGenerations :execrows
DELETE FROM generations AS generation
WHERE NOT EXISTS (
  SELECT 1
  FROM catalog
  WHERE generation.id = active_generation_id
     OR generation.id = previous_generation_id
     OR generation.id = staging_generation_id
);

-- name: GenerationCount :one
SELECT count(*) FROM generations;
