-- One row per delivered notification. seq is monotonic per user_id and
-- is what powers the "resume from sequence X" recovery in Phase 5.
CREATE TABLE IF NOT EXISTS notifications (
    id          UUID        PRIMARY KEY,
    user_id     TEXT        NOT NULL,
    seq         BIGINT      NOT NULL,
    title       TEXT        NOT NULL,
    body        TEXT        NOT NULL,
    data        JSONB       NOT NULL DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    read_at     TIMESTAMPTZ,
    UNIQUE (user_id, seq)
);
CREATE INDEX IF NOT EXISTS idx_notifications_user_unread
    ON notifications (user_id, created_at DESC) WHERE read_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_notifications_user_seq
    ON notifications (user_id, seq DESC);

-- Lightweight per-user counter to allocate the next seq atomically.
-- Updated via UPSERT inside the same transaction as the INSERT into
-- notifications, so a duplicate publish can't reuse a seq.
CREATE TABLE IF NOT EXISTS user_offsets (
    user_id   TEXT PRIMARY KEY,
    last_seq  BIGINT NOT NULL DEFAULT 0
);
