package pgstore

import "fmt"

const defaultEmbeddingDimension = 1536

func buildSchemaSQL(dim int) string {
	if dim <= 0 {
		dim = defaultEmbeddingDimension
	}
	return fmt.Sprintf(`
CREATE EXTENSION IF NOT EXISTS vector;

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

    hit_count         INTEGER DEFAULT 0,
    useful_count      INTEGER DEFAULT 0,
    last_hit_at       TIMESTAMPTZ,

    created_at        TIMESTAMPTZ DEFAULT NOW(),
    updated_at        TIMESTAMPTZ DEFAULT NOW(),

    PRIMARY KEY (owner_id, id)
);

CREATE INDEX IF NOT EXISTS idx_knowledge_owner ON knowledge(owner_id);
CREATE INDEX IF NOT EXISTS idx_knowledge_owner_status ON knowledge(owner_id, status);
CREATE INDEX IF NOT EXISTS idx_knowledge_owner_tags ON knowledge USING GIN(tags);
CREATE INDEX IF NOT EXISTS idx_knowledge_owner_created ON knowledge(owner_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_knowledge_embedding ON knowledge USING hnsw (embedding vector_cosine_ops);
CREATE INDEX IF NOT EXISTS idx_knowledge_owner_type ON knowledge(owner_id, knowledge_type);

CREATE TABLE IF NOT EXISTS search_log (
    id           TEXT NOT NULL,
    owner_id     TEXT NOT NULL DEFAULT 'default',
    query        TEXT NOT NULL,
    result_count INTEGER DEFAULT 0,
    had_feedback BOOLEAN DEFAULT FALSE,
    created_at   TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (owner_id, id)
);

CREATE INDEX IF NOT EXISTS idx_search_log_owner_created ON search_log(owner_id, created_at DESC);
`, dim)
}

const migrationSQL = `
ALTER TABLE knowledge ADD COLUMN IF NOT EXISTS knowledge_type TEXT DEFAULT 'general';
CREATE INDEX IF NOT EXISTS idx_knowledge_owner_type ON knowledge(owner_id, knowledge_type);
ALTER TABLE knowledge ADD COLUMN IF NOT EXISTS superseded_by TEXT DEFAULT '';
ALTER TABLE knowledge ADD COLUMN IF NOT EXISTS knowledge_category TEXT DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_knowledge_owner_category ON knowledge(owner_id, knowledge_category);
`
