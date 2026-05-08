import { useEffect } from "react";

import { useAppDispatch, useAppSelector } from "@/store";
import { setInitial } from "@/store/notificationsSlice";
import { api } from "@/lib/api";
import type { NotificationListResponse } from "@/lib/types";
import { Header } from "@/components/layout/Header";
import { NotificationList } from "@/components/notifications/NotificationList";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";

export function Dashboard() {
  const dispatch = useAppDispatch();
  const user = useAppSelector((s) => s.auth.user);
  const unread = useAppSelector((s) => s.notifications.unreadCount);
  const lastSeq = useAppSelector((s) => s.notifications.lastSeenSeq);

  // Initial load on mount — gives the user their existing notifications
  // immediately. The WS hook then takes over for live updates.
  useEffect(() => {
    if (!user) return;
    api<NotificationListResponse>("/v1/notifications?limit=50")
      .then((res) =>
        dispatch(
          setInitial({
            items: res.data,
            unreadCount: res.unread_count,
          }),
        ),
      )
      .catch((err) => console.warn("initial fetch failed", err));
  }, [user, dispatch]);

  return (
    <div className="min-h-full">
      <Header />
      <main className="mx-auto w-full max-w-6xl px-4 py-10 sm:px-6 lg:px-8">
        <div className="mb-8 flex items-end justify-between">
          <div>
            <h1 className="text-2xl font-semibold tracking-tight">
              Welcome back, {user?.username}
            </h1>
            <p className="text-sm text-muted-foreground">
              Notifications stream in over a single WebSocket connection. Try the
              admin panel in another tab to see them arrive in real time.
            </p>
          </div>
        </div>

        <div className="mb-6 grid gap-3 sm:grid-cols-3">
          <Card>
            <CardHeader>
              <CardTitle className="text-sm uppercase tracking-wide text-muted-foreground">
                Unread
              </CardTitle>
            </CardHeader>
            <CardContent>
              <p className="text-3xl font-semibold">{unread}</p>
            </CardContent>
          </Card>
          <Card>
            <CardHeader>
              <CardTitle className="text-sm uppercase tracking-wide text-muted-foreground">
                Last seq
              </CardTitle>
            </CardHeader>
            <CardContent>
              <p className="font-mono text-3xl font-semibold">{lastSeq}</p>
            </CardContent>
          </Card>
          <Card>
            <CardHeader>
              <CardTitle className="text-sm uppercase tracking-wide text-muted-foreground">
                Role
              </CardTitle>
            </CardHeader>
            <CardContent>
              <Badge variant={user?.role === "admin" ? "default" : "secondary"}>
                {user?.role ?? "—"}
              </Badge>
            </CardContent>
          </Card>
        </div>

        <NotificationList />
      </main>
    </div>
  );
}
