import { motion, AnimatePresence } from "framer-motion";
import { Check, Inbox } from "lucide-react";

import { useAppDispatch, useAppSelector } from "@/store";
import { markRead } from "@/store/notificationsSlice";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { api } from "@/lib/api";
import { timeAgo, cn } from "@/lib/utils";

export function NotificationList() {
  const dispatch = useAppDispatch();
  const items = useAppSelector((s) => s.notifications.items);

  if (items.length === 0) {
    return (
      <Card>
        <CardContent className="flex flex-col items-center gap-3 p-12 text-center text-muted-foreground">
          <Inbox className="h-8 w-8" />
          <div>
            <p className="font-medium text-foreground">No notifications yet</p>
            <p className="text-sm">When admin sends one, it'll appear here in real time.</p>
          </div>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Inbox</CardTitle>
      </CardHeader>
      <CardContent className="p-0">
        <ul className="divide-y">
          <AnimatePresence initial={false}>
            {items.map((n) => (
              <motion.li
                key={n.id}
                layout
                initial={{ opacity: 0, x: -16 }}
                animate={{ opacity: 1, x: 0 }}
                exit={{ opacity: 0 }}
                className={cn("flex items-start gap-4 px-6 py-4", !n.read_at && "bg-accent/40")}
              >
                <div className="min-w-0 flex-1">
                  <p className={cn("font-medium", n.read_at && "text-muted-foreground")}>
                    {n.title}
                  </p>
                  {n.body && <p className="mt-0.5 text-sm text-muted-foreground">{n.body}</p>}
                  <p className="mt-1 text-xs text-muted-foreground">
                    seq {n.seq} · {timeAgo(n.created_at)}
                  </p>
                </div>
                {!n.read_at && (
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={async () => {
                      try {
                        await api(`/v1/notifications/${n.id}/read`, { method: "POST" });
                        dispatch(markRead(n.id));
                      } catch (err) {
                        console.warn("mark read failed", err);
                      }
                    }}
                  >
                    <Check className="h-4 w-4" />
                    Mark read
                  </Button>
                )}
              </motion.li>
            ))}
          </AnimatePresence>
        </ul>
      </CardContent>
    </Card>
  );
}
