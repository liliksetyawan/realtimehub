export interface User {
  id: string;
  username: string;
  role: "user" | "admin";
}

export interface LoginResponse {
  token: string;
  expires_at: string;
  user_id: string;
  username: string;
  role: "user" | "admin";
}

export interface Notification {
  id: string;
  user_id: string;
  seq: number;
  title: string;
  body: string;
  data?: Record<string, string>;
  created_at: string;
  read_at?: string;
}

export interface NotificationListResponse {
  data: Notification[];
  total: number;
  unread_count: number;
}

// WebSocket wire frames — must mirror Go's websocket.Frame.
export type WsMsgType =
  | "welcome"
  | "notification"
  | "pong"
  | "error"
  | "server_ping"
  | "ping"
  | "ack"
  | "resume";

export interface WsFrame {
  type: WsMsgType;
  seq?: number;
  payload?: unknown;
}

export interface WelcomePayload {
  user_id: string;
  conn_id: string;
  current_seq: number;
  acked_seq: number;
  server_time: string;
}

export interface NotificationPayload {
  id: string;
  title: string;
  body: string;
  data?: Record<string, string>;
  created_at: string;
}

export interface AckPayload {
  up_to_seq: number;
}
