// Thin fetch wrapper that injects the bearer token from localStorage and
// surfaces typed errors. Sticking with fetch (not RTK Query) here because
// most of the live state arrives via WebSocket; REST is just for login,
// initial list, mark-read, and admin send.

const API_BASE_URL =
  import.meta.env.VITE_API_BASE_URL ?? "http://localhost:8090";

const TOKEN_KEY = "realtimehub:token";

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY);
}

export function setToken(t: string | null) {
  if (t) localStorage.setItem(TOKEN_KEY, t);
  else localStorage.removeItem(TOKEN_KEY);
}

export class ApiError extends Error {
  constructor(public status: number, public code: string, message: string) {
    super(message);
  }
}

export async function api<T = unknown>(
  path: string,
  init: RequestInit = {},
): Promise<T> {
  const token = getToken();
  const headers = new Headers(init.headers ?? {});
  headers.set("Content-Type", "application/json");
  if (token) headers.set("Authorization", `Bearer ${token}`);

  const res = await fetch(`${API_BASE_URL}${path}`, { ...init, headers });

  if (res.status === 204) return undefined as T;

  let body: unknown = null;
  try {
    body = await res.json();
  } catch {
    /* not json */
  }

  if (!res.ok) {
    const errBody = body as { error?: string; message?: string } | null;
    throw new ApiError(
      res.status,
      errBody?.error ?? "request_failed",
      errBody?.message ?? res.statusText,
    );
  }
  return body as T;
}

export const wsURL = (token: string) => {
  const base = import.meta.env.VITE_WS_URL ?? "ws://localhost:8090";
  return `${base}/ws?token=${encodeURIComponent(token)}`;
};

export { API_BASE_URL };
