-- Tracks the highest seq the *server* has confirmed the client received,
-- per user. Updated when the client sends a {type: "ack", up_to_seq: N}
-- frame after rendering a notification.
--
-- On reconnect, the WS server takes MAX(client's reported last_seen_seq,
-- delivery_offsets.last_acked_seq) as the resume cursor — server-
-- authoritative if the client claims less than it actually acked, and
-- client-authoritative if the client has truly moved ahead in some way
-- the server hasn't seen yet.
--
-- The UPSERT in the repo uses GREATEST() so out-of-order ack frames
-- can't accidentally decrement the offset.
CREATE TABLE IF NOT EXISTS delivery_offsets (
    user_id        TEXT        PRIMARY KEY,
    last_acked_seq BIGINT      NOT NULL DEFAULT 0,
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
