-- +goose Up
CREATE TABLE vault_notes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    path TEXT UNIQUE NOT NULL,           -- relative path from vault root
    filename TEXT NOT NULL,

    -- Parsed frontmatter (common Obsidian fields)
    title TEXT,
    tags TEXT[],                         -- parsed from tags: or #inline-tags
    aliases TEXT[],
    created_at TIMESTAMPTZ,              -- from frontmatter or file stat
    modified_at TIMESTAMPTZ,             -- from frontmatter or file stat
    publish BOOLEAN DEFAULT false,

    -- Flexible frontmatter storage for custom fields
    frontmatter JSONB DEFAULT '{}',

    -- Content
    body TEXT,                           -- markdown without frontmatter
    raw_content TEXT,                    -- original file content

    -- Sync metadata
    content_hash TEXT NOT NULL,          -- SHA256 for change detection
    file_size_bytes BIGINT,
    synced_at TIMESTAMPTZ DEFAULT NOW(),

    -- Obsidian-specific
    outgoing_links TEXT[],               -- [[extracted]] wikilinks

    CONSTRAINT valid_path CHECK (path ~ '^[^/].*\.md$')
);

CREATE INDEX idx_notes_tags ON vault_notes USING GIN (tags);
CREATE INDEX idx_notes_frontmatter ON vault_notes USING GIN (frontmatter);
CREATE INDEX idx_notes_modified ON vault_notes (modified_at DESC);
CREATE INDEX idx_notes_hash ON vault_notes (content_hash);

-- +goose Down
DROP TABLE vault_notes;
