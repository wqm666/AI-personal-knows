package pgstore

import "fmt"

const defaultEmbeddingDimension = 1536

func buildSchemaSQL(dim int) string {
	if dim <= 0 {
		dim = defaultEmbeddingDimension
	}
	return fmt.Sprintf(`
CREATE EXTENSION IF NOT EXISTS vector;
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE IF NOT EXISTS knowledge (
    id                TEXT NOT NULL,
    owner_id          TEXT NOT NULL DEFAULT 'default',
    title             TEXT NOT NULL,
    content           TEXT NOT NULL,
    summary           TEXT DEFAULT '',

    source            TEXT NOT NULL DEFAULT 'manual',
    source_ref        TEXT DEFAULT '',

    knowledge_type    TEXT DEFAULT 'general',

    tags              TEXT[] DEFAULT '{}',

    embedding         vector(%d),

    related_ids       TEXT[] DEFAULT '{}',
    status            TEXT DEFAULT 'active',
    consolidated_from TEXT[] DEFAULT '{}',

    review_status     TEXT DEFAULT 'pending',
    confidence        INTEGER DEFAULT 0,
    review_reason     TEXT DEFAULT '',
    reviewed_at       TIMESTAMPTZ,

    hit_count         INTEGER DEFAULT 0,
    useful_count      INTEGER DEFAULT 0,
    last_hit_at       TIMESTAMPTZ,

    created_at        TIMESTAMPTZ DEFAULT NOW(),
    updated_at        TIMESTAMPTZ DEFAULT NOW(),

    PRIMARY KEY (owner_id, id)
);

CREATE TABLE IF NOT EXISTS search_log (
    id               TEXT NOT NULL,
    owner_id         TEXT NOT NULL DEFAULT 'default',
    query            TEXT NOT NULL,
    source           TEXT DEFAULT 'api',
    result_count     INTEGER DEFAULT 0,
    result_ids       TEXT[] DEFAULT '{}',
    had_feedback     BOOLEAN DEFAULT FALSE,
    feedback_bad_ids TEXT[] DEFAULT '{}',
    created_at       TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (owner_id, id)
);
CREATE TABLE IF NOT EXISTS worklog (
    id         TEXT NOT NULL,
    owner_id   TEXT NOT NULL DEFAULT 'default',
    date       TEXT NOT NULL,
    content    TEXT NOT NULL,
    project    TEXT DEFAULT '',
    tags       TEXT[] DEFAULT '{}',
    duration   INTEGER DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (owner_id, id)
);
CREATE INDEX IF NOT EXISTS idx_worklog_owner_date ON worklog(owner_id, date DESC);
`, dim)
}

const migrationSQL = `
-- Add columns for old tables (IF NOT EXISTS makes these idempotent)
ALTER TABLE knowledge ADD COLUMN IF NOT EXISTS knowledge_type TEXT DEFAULT 'general';
ALTER TABLE knowledge ADD COLUMN IF NOT EXISTS superseded_by TEXT DEFAULT '';
ALTER TABLE knowledge ADD COLUMN IF NOT EXISTS knowledge_category TEXT DEFAULT '';
ALTER TABLE knowledge ADD COLUMN IF NOT EXISTS review_status TEXT DEFAULT 'pending';
ALTER TABLE knowledge ADD COLUMN IF NOT EXISTS confidence INTEGER DEFAULT 0;
ALTER TABLE knowledge ADD COLUMN IF NOT EXISTS review_reason TEXT DEFAULT '';
ALTER TABLE knowledge ADD COLUMN IF NOT EXISTS reviewed_at TIMESTAMPTZ;

-- All indexes (safe for both new and migrated tables)
CREATE INDEX IF NOT EXISTS idx_knowledge_owner ON knowledge(owner_id);
CREATE INDEX IF NOT EXISTS idx_knowledge_owner_status ON knowledge(owner_id, status);
CREATE INDEX IF NOT EXISTS idx_knowledge_owner_tags ON knowledge USING GIN(tags);
CREATE INDEX IF NOT EXISTS idx_knowledge_owner_created ON knowledge(owner_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_knowledge_embedding ON knowledge USING hnsw (embedding vector_cosine_ops);
CREATE INDEX IF NOT EXISTS idx_knowledge_owner_type ON knowledge(owner_id, knowledge_type);
CREATE INDEX IF NOT EXISTS idx_knowledge_owner_category ON knowledge(owner_id, knowledge_category);
CREATE INDEX IF NOT EXISTS idx_knowledge_owner_review ON knowledge(owner_id, review_status);
CREATE INDEX IF NOT EXISTS idx_knowledge_title_trgm ON knowledge USING GIN(title gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_knowledge_content_trgm ON knowledge USING GIN(content gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_search_log_owner_created ON search_log(owner_id, created_at DESC);
ALTER TABLE search_log ADD COLUMN IF NOT EXISTS source TEXT DEFAULT 'api';
ALTER TABLE search_log ADD COLUMN IF NOT EXISTS result_ids TEXT[] DEFAULT '{}';
ALTER TABLE search_log ADD COLUMN IF NOT EXISTS feedback_bad_ids TEXT[] DEFAULT '{}';
`
