import { useEffect, useState } from "react";
import { Navigate } from "react-router-dom";
import { Send } from "lucide-react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Header } from "@/components/layout/Header";
import { Badge } from "@/components/ui/badge";
import { useAppSelector } from "@/store";
import { api } from "@/lib/api";

interface DemoUser {
  id: string;
  username: string;
  role: "user" | "admin";
}

export function Admin() {
  const user = useAppSelector((s) => s.auth.user);
  const [users, setUsers] = useState<DemoUser[]>([]);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [title, setTitle] = useState("Heads up");
  const [body, setBody] = useState("Saga has just confirmed your order.");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (user?.role !== "admin") return;
    api<{ data: DemoUser[] }>("/v1/admin/users")
      .then((r) => {
        setUsers(r.data);
        setSelected(new Set(r.data.map((u) => u.id)));
      })
      .catch((err) => toast.error("Couldn't load users", { description: (err as Error).message }));
  }, [user]);

  if (user?.role !== "admin") return <Navigate to="/" replace />;

  const toggle = (id: string) => {
    const next = new Set(selected);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    setSelected(next);
  };

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (selected.size === 0) {
      toast.error("Pick at least one recipient");
      return;
    }
    setBusy(true);
    try {
      const res = await api<{ sent: { user_id: string; seq: number }[] }>(
        "/v1/admin/notifications",
        {
          method: "POST",
          body: JSON.stringify({
            user_ids: Array.from(selected),
            title,
            body,
          }),
        },
      );
      toast.success(`Sent to ${res.sent.length} user${res.sent.length === 1 ? "" : "s"}`);
    } catch (err) {
      toast.error("Send failed", { description: (err as Error).message });
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="min-h-full">
      <Header />
      <main className="mx-auto w-full max-w-3xl px-4 py-10 sm:px-6 lg:px-8">
        <Card>
          <CardHeader>
            <CardTitle className="text-xl">Send notification</CardTitle>
            <CardDescription>
              POST <code className="font-mono">/v1/admin/notifications</code>. Each
              recipient gets its own row in postgres + a real-time push if any of
              their tabs are open.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <form onSubmit={submit} className="space-y-6">
              <div className="space-y-2">
                <Label htmlFor="title">Title</Label>
                <Input
                  id="title"
                  value={title}
                  onChange={(e) => setTitle(e.target.value)}
                  required
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="body">Body</Label>
                <Input id="body" value={body} onChange={(e) => setBody(e.target.value)} />
              </div>
              <div>
                <Label className="mb-2 block">Recipients</Label>
                <div className="flex flex-wrap gap-2">
                  {users.map((u) => {
                    const active = selected.has(u.id);
                    return (
                      <button
                        type="button"
                        key={u.id}
                        onClick={() => toggle(u.id)}
                        className={`rounded-full border px-3 py-1 text-xs font-medium transition-colors ${
                          active
                            ? "border-primary bg-primary text-primary-foreground"
                            : "border-border bg-background text-muted-foreground hover:text-foreground"
                        }`}
                      >
                        {u.username}
                      </button>
                    );
                  })}
                </div>
                <div className="mt-3 flex items-center gap-2 text-xs text-muted-foreground">
                  <Badge variant="outline" className="font-normal">{selected.size} selected</Badge>
                  <button
                    type="button"
                    className="underline hover:text-foreground"
                    onClick={() => setSelected(new Set(users.map((u) => u.id)))}
                  >
                    select all
                  </button>
                  <button
                    type="button"
                    className="underline hover:text-foreground"
                    onClick={() => setSelected(new Set())}
                  >
                    clear
                  </button>
                </div>
              </div>
              <Button type="submit" size="lg" className="w-full" disabled={busy}>
                <Send className="h-4 w-4" />
                {busy ? "Sending…" : "Send notification"}
              </Button>
            </form>
          </CardContent>
        </Card>
      </main>
    </div>
  );
}
