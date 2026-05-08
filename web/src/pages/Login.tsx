import { useState } from "react";
import { Navigate, useNavigate } from "react-router-dom";
import { Radio } from "lucide-react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { useAppDispatch, useAppSelector } from "@/store";
import { setSession } from "@/store/authSlice";
import { api, setToken } from "@/lib/api";
import type { LoginResponse } from "@/lib/types";

export function Login() {
  const dispatch = useAppDispatch();
  const nav = useNavigate();
  const isAuthed = useAppSelector((s) => !!s.auth.token);
  const [username, setUsername] = useState("alice");
  const [password, setPassword] = useState("password123");
  const [busy, setBusy] = useState(false);

  if (isAuthed) return <Navigate to="/" replace />;

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    try {
      const res = await api<LoginResponse>("/v1/auth/login", {
        method: "POST",
        body: JSON.stringify({ username, password }),
      });
      setToken(res.token);
      dispatch(
        setSession({
          token: res.token,
          user: { id: res.user_id, username: res.username, role: res.role },
        }),
      );
      toast.success(`Welcome, ${res.username}`);
      nav("/");
    } catch (err) {
      toast.error("Login failed", {
        description: (err as Error).message,
      });
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="relative grid min-h-full place-items-center px-4">
      <div className="absolute inset-x-0 top-0 -z-10 h-[420px] bg-radial-fade" />
      <div className="absolute inset-x-0 top-0 -z-10 h-[420px] bg-grid opacity-30" />

      <div className="w-full max-w-sm">
        <div className="mb-8 flex items-center gap-2 font-semibold">
          <span className="grid h-9 w-9 place-items-center rounded-lg bg-primary text-primary-foreground shadow-sm">
            <Radio className="h-5 w-5" />
          </span>
          <span className="text-xl tracking-tight">RealtimeHub</span>
        </div>

        <Card>
          <CardHeader>
            <CardTitle className="text-xl">Sign in</CardTitle>
            <CardDescription>
              Demo users:{" "}
              <code className="font-mono text-xs">alice / password123</code>,{" "}
              <code className="font-mono text-xs">admin / admin123</code>
            </CardDescription>
          </CardHeader>
          <CardContent>
            <form onSubmit={submit} className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="username">Username</Label>
                <Input
                  id="username"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  autoFocus
                  required
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="password">Password</Label>
                <Input
                  id="password"
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  required
                />
              </div>
              <Button type="submit" className="w-full" disabled={busy}>
                {busy ? "Signing in…" : "Sign in"}
              </Button>
            </form>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
