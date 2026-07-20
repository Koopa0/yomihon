-- Semantic-generation schema v1. This is both sqlc build-time truth and the
-- embedded runtime bootstrap. Upgrades do not mutate this file in place: a
-- future compatible version must ship an explicit-build copy-forward importer
-- before raising user_version; an unknown version is rebuilt explicitly.

CREATE TABLE IF NOT EXISTS generations (
  -- Monotonic local identifier. AUTOINCREMENT prevents a deleted staging ID
  -- from being reused while an older in-process Build handle still exists.
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  -- Invalidates generations when their durable representation changes.
  vector_format_version INTEGER NOT NULL CHECK (vector_format_version > 0),
  -- Provider model whose numerical vector space owns every chunk below.
  model TEXT NOT NULL CHECK (length(model) > 0),
  -- Exact count of little-endian float32 values in each vector.
  dimension INTEGER NOT NULL CHECK (dimension > 0),
  -- SHA-256 of request templates, cardinality, and response handling.
  protocol_epoch BLOB NOT NULL
    CHECK (typeof(protocol_epoch) = 'blob' AND length(protocol_epoch) = 32),
  -- SHA-256 of deterministic local chunking and preprocessing rules.
  chunker_epoch BLOB NOT NULL
    CHECK (typeof(chunker_epoch) = 'blob' AND length(chunker_epoch) = 32),
  -- Canonical absolute vault path whose corpus this generation represents.
  vault_root TEXT NOT NULL CHECK (length(vault_root) > 0),
  -- SHA-256 of the exact artifact and privacy eligibility authority.
  corpus_policy_fingerprint BLOB NOT NULL
    CHECK (
      typeof(corpus_policy_fingerprint) = 'blob' AND
      length(corpus_policy_fingerprint) = 32
    ),
  -- SHA-256 of the exact contract source bytes used by this build. It gates
  -- staging resume but is not part of numerical/cache compatibility.
  policy_source_fingerprint BLOB NOT NULL
    CHECK (
      typeof(policy_source_fingerprint) = 'blob' AND
      length(policy_source_fingerprint) = 32
    ),
  -- SHA-256 of the sorted complete chunk manifest targeted by this build. This
  -- deliberate derived integrity witness is recomputed on resume/activation.
  target_corpus_fingerprint BLOB NOT NULL
    CHECK (
      typeof(target_corpus_fingerprint) = 'blob' AND
      length(target_corpus_fingerprint) = 32
    ),
  -- Derived manifest count, deliberately duplicated as an integrity witness
  -- and cross-checked against persisted target rows before activation.
  expected_chunks INTEGER NOT NULL CHECK (expected_chunks >= 0),
  -- Measured exact-scan top-k p95 in integer microseconds; zero until measured.
  top_k_p95_us INTEGER NOT NULL DEFAULT 0 CHECK (top_k_p95_us >= 0)
) STRICT;

CREATE TABLE IF NOT EXISTS catalog (
  -- Enforces the store's one role catalog.
  singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
  -- Complete generation served to semantic search; NULL before first build.
  active_generation_id INTEGER REFERENCES generations(id),
  -- Prior active generation retained for inspection only, never auto-served.
  previous_generation_id INTEGER REFERENCES generations(id),
  -- Sole mutable generation; NULL when no build is resumable.
  staging_generation_id INTEGER REFERENCES generations(id),
  CHECK (
    active_generation_id IS NULL OR previous_generation_id IS NULL OR
    active_generation_id <> previous_generation_id
  ),
  CHECK (
    active_generation_id IS NULL OR staging_generation_id IS NULL OR
    active_generation_id <> staging_generation_id
  ),
  CHECK (
    previous_generation_id IS NULL OR staging_generation_id IS NULL OR
    previous_generation_id <> staging_generation_id
  )
) STRICT;

-- One row per note represented by a generation. Keeping note_hash here avoids
-- a partial dependency on the chunk ordinal and makes the schema third-normal-
-- form: note identity depends on (generation_id, rel_path), while chunk
-- identity below depends on (generation_id, rel_path, ordinal).
CREATE TABLE IF NOT EXISTS notes (
  -- Generation that owns this note identity.
  generation_id INTEGER NOT NULL
    REFERENCES generations(id) ON DELETE CASCADE,
  -- Slash-form vault-relative path; current local bytes supply display evidence.
  rel_path TEXT NOT NULL CHECK (length(rel_path) > 0),
  -- SHA-256 of the complete note bytes represented by every child chunk.
  note_hash BLOB NOT NULL
    CHECK (typeof(note_hash) = 'blob' AND length(note_hash) = 32),
  PRIMARY KEY (generation_id, rel_path)
) STRICT;

CREATE TABLE IF NOT EXISTS chunks (
  -- Generation and note that exclusively own this immutable-or-staging row.
  generation_id INTEGER NOT NULL
    REFERENCES generations(id) ON DELETE CASCADE,
  -- Slash-form vault-relative path joined to notes for its complete note hash.
  rel_path TEXT NOT NULL CHECK (length(rel_path) > 0),
  -- Zero-based deterministic chunk ordinal inside the note.
  ordinal INTEGER NOT NULL CHECK (ordinal BETWEEN 0 AND 4294967295),
  -- SHA-256 of the exact document bytes submitted to the provider.
  submitted_hash BLOB NOT NULL
    CHECK (typeof(submitted_hash) = 'blob' AND length(submitted_hash) = 32),
  -- Little-endian IEEE-754 float32 values. NULL means this exact staging target
  -- is still pending; active and previous generations may never retain NULL.
  vector BLOB CHECK (
    vector IS NULL OR
    (typeof(vector) = 'blob' AND length(vector) >= 4 AND length(vector) % 4 = 0)
  ),
  PRIMARY KEY (generation_id, rel_path, ordinal),
  FOREIGN KEY (generation_id, rel_path)
    REFERENCES notes(generation_id, rel_path) ON DELETE CASCADE
) STRICT;

-- Exact submitted bytes are the reusable numerical input within one complete
-- embedding identity. Path, note hash, and ordinal describe the new manifest
-- row and do not belong in this lookup key.
CREATE INDEX IF NOT EXISTS chunks_submitted_hash
  ON chunks(generation_id, submitted_hash, rel_path, ordinal);

CREATE TABLE IF NOT EXISTS attempts (
  -- Staging generation whose paid retry budget this reservation consumes.
  generation_id INTEGER NOT NULL
    REFERENCES generations(id) ON DELETE CASCADE,
  -- Exact pending target chunk path. The complete target manifest is committed
  -- before any reservation or provider request.
  rel_path TEXT NOT NULL CHECK (length(rel_path) > 0),
  -- Target chunk ordinal paired with rel_path.
  ordinal INTEGER NOT NULL CHECK (ordinal BETWEEN 0 AND 4294967295),
  -- Durable send-slot reservations, capped before a sixth capability invocation.
  attempts INTEGER NOT NULL CHECK (attempts BETWEEN 1 AND 5),
  -- Earliest Unix millisecond eligible for another attempt; NULL means now.
  retry_not_before_unix_ms INTEGER
    CHECK (retry_not_before_unix_ms IS NULL OR retry_not_before_unix_ms >= 0),
  PRIMARY KEY (generation_id, rel_path, ordinal),
  FOREIGN KEY (generation_id, rel_path, ordinal)
    REFERENCES chunks(generation_id, rel_path, ordinal) ON DELETE CASCADE
) STRICT;

-- SQLite's application-owned schema marker. Unlike a catalog column, this can
-- be read before sqlc queries assume the current table shape.
PRAGMA user_version = 1;
