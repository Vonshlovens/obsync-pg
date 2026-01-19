-- +goose Up
CREATE TABLE vault_attachments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    path TEXT UNIQUE NOT NULL,
    filename TEXT NOT NULL,
    extension TEXT,
    mime_type TEXT,
    file_size_bytes BIGINT,
    content_hash TEXT NOT NULL,

    -- Store binary data (for files < 50MB; skip larger)
    data BYTEA,

    synced_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_attachments_ext ON vault_attachments (extension);
CREATE INDEX idx_attachments_hash ON vault_attachments (content_hash);

-- +goose Down
DROP TABLE vault_attachments;
