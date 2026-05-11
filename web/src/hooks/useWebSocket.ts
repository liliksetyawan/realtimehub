import { useEffect, useRef } from "react";
import { toast } from "sonner";

import { useAppDispatch, useAppSelector } from "@/store";
import { receive as receiveNotif } from "@/store/notificationsSlice";
import { setStatus, bumpReconnect, touch } from "@/store/wsSlice";
import { wsURL } from "@/lib/api";
import type {
  NotificationPayload,
  WelcomePayload,
  WsFrame,
} from "@/lib/types";

/**
 * useWebSocket owns the entire client-side connection lifecycle:
 *   - opens a WebSocket with the JWT in the query string
 *   - maintains heartbeat (responds to server_ping with pong)
 *   - reconnects with exponential backoff + jitter (max 30s)
 *   - resumes from lastSeenSeq on reconnect (replays missed notifs)
 *   - dispatches Redux actions for received notifications
 *
 * Hook expects there to be exactly one instance mounted (typically in
 * App, gated by auth). Multiple instances would open multiple sockets.
 */
export function useWebSocket() {
  const dispatch = useAppDispatch();
  const token = useAppSelector((s) => s.auth.token);
  const lastSeenSeq = useAppSelector((s) => s.notifications.lastSeenSeq);

  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimer = useRef<number | null>(null);
  const attemptRef = useRef(0);
  const seqRef = useRef(0);
  // Mirror lastSeenSeq into a ref so the resume payload uses fresh state
  // without re-running the connect effect.
  seqRef.current = lastSeenSeq;

  useEffect(() => {
    if (!token) return;

    let stopped = false;

    const send = (frame: WsFrame) => {
      const ws = wsRef.current;
      if (ws && ws.readyState === WebSocket.OPEN) ws.send(JSON.stringify(frame));
    };

    const connect = () => {
      if (stopped) return;
      dispatch(setStatus(attemptRef.current === 0 ? "connecting" : "reconnecting"));
      const ws = new WebSocket(wsURL(token));
      wsRef.current = ws;

      ws.onopen = () => {
        attemptRef.current = 0;
        dispatch(setStatus("open"));
        // If we've seen any seqs before this connect, ask the server to
        // replay anything we missed.
        if (seqRef.current > 0) {
          send({ type: "resume", payload: { from_seq: seqRef.current } });
        }
      };

      ws.onmessage = (ev) => {
        dispatch(touch());
        let frame: WsFrame;
        try {
          frame = JSON.parse(ev.data);
        } catch {
          return;
        }
        switch (frame.type) {
          case "welcome": {
            const p = frame.payload as WelcomePayload;
            // If server's seq is ahead of ours, resume to catch up.
            if (p.current_seq > seqRef.current) {
              send({ type: "resume", payload: { from_seq: seqRef.current } });
            }
            break;
          }
          case "notification": {
            const p = frame.payload as NotificationPayload;
            dispatch(
              receiveNotif({
                id: p.id,
                user_id: "",
                seq: frame.seq ?? 0,
                title: p.title,
                body: p.body,
                data: p.data,
                created_at: p.created_at,
              }),
            );
            toast(p.title, { description: p.body });
            // Confirm receipt so the server's delivery_offsets table
            // reflects what this client has rendered. The server is
            // monotonic — out-of-order acks don't lower the offset.
            if (frame.seq && frame.seq > 0) {
              send({ type: "ack", payload: { up_to_seq: frame.seq } });
            }
            break;
          }
          case "server_ping":
            send({ type: "pong" });
            break;
          case "pong":
            // Server responding to our ping; just bump touch (already done).
            break;
          case "error":
            console.warn("[ws] server error frame", frame.payload);
            break;
        }
      };

      ws.onclose = () => {
        if (stopped) return;
        dispatch(setStatus("closed"));
        scheduleReconnect();
      };

      ws.onerror = () => {
        // onclose will follow; let scheduleReconnect handle backoff there.
      };
    };

    const scheduleReconnect = () => {
      if (stopped) return;
      attemptRef.current += 1;
      dispatch(bumpReconnect());
      // Exponential backoff with full jitter, capped at 30s.
      const expo = Math.min(30_000, 1000 * 2 ** Math.min(attemptRef.current - 1, 5));
      const delay = Math.floor(Math.random() * expo);
      reconnectTimer.current = window.setTimeout(connect, delay);
    };

    connect();

    return () => {
      stopped = true;
      if (reconnectTimer.current) window.clearTimeout(reconnectTimer.current);
      wsRef.current?.close();
      dispatch(setStatus("idle"));
    };
    // We intentionally don't include lastSeenSeq in deps — the seqRef
    // above keeps it fresh without forcing a reconnect.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [token, dispatch]);
}

