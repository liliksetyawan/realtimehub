import { useEffect } from "react";
import { Navigate, Route, Routes } from "react-router-dom";

import { useAppDispatch, useAppSelector } from "@/store";
import { setSession } from "@/store/authSlice";
import { api, getToken, setToken } from "@/lib/api";
import { useWebSocket } from "@/hooks/useWebSocket";

import { Dashboard } from "@/pages/Dashboard";
import { Admin } from "@/pages/Admin";
import { Login } from "@/pages/Login";
import { NotFound } from "@/pages/NotFound";

interface MeResponse {
  user_id: string;
  username: string;
  role: "user" | "admin";
}

function RequireAuth({ children }: { children: React.ReactNode }) {
  const token = useAppSelector((s) => s.auth.token);
  return token ? <>{children}</> : <Navigate to="/login" replace />;
}

export function App() {
  const dispatch = useAppDispatch();
  const token = useAppSelector((s) => s.auth.token);
  // Drives WS connection lifecycle. No-op when token is empty.
  useWebSocket();

  // On boot, restore session from localStorage if any. /v1/me validates the
  // token and gives us the user back.
  useEffect(() => {
    if (token) return;
    const cached = getToken();
    if (!cached) return;
    api<MeResponse>("/v1/me")
      .then((me) =>
        dispatch(
          setSession({
            token: cached,
            user: { id: me.user_id, username: me.username, role: me.role },
          }),
        ),
      )
      .catch(() => setToken(null));
  }, [token, dispatch]);

  return (
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route
        path="/"
        element={
          <RequireAuth>
            <Dashboard />
          </RequireAuth>
        }
      />
      <Route
        path="/admin"
        element={
          <RequireAuth>
            <Admin />
          </RequireAuth>
        }
      />
      <Route path="*" element={<NotFound />} />
    </Routes>
  );
}
