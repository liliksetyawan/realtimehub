import { useAppSelector } from "@/store";
import { cn } from "@/lib/utils";

const COPY: Record<string, { label: string; dot: string }> = {
  idle: { label: "Idle", dot: "bg-muted-foreground" },
  connecting: { label: "Connecting…", dot: "bg-yellow-500 animate-pulse" },
  open: { label: "Connected", dot: "bg-success" },
  reconnecting: { label: "Reconnecting…", dot: "bg-yellow-500 animate-pulse" },
  closed: { label: "Disconnected", dot: "bg-destructive" },
};

export function ConnectionStatus() {
  const { status, reconnectAttempts } = useAppSelector((s) => s.ws);
  const c = COPY[status] ?? COPY.idle;
  return (
    <div className="inline-flex items-center gap-2 rounded-full border bg-card px-3 py-1 text-xs">
      <span className={cn("h-1.5 w-1.5 rounded-full", c.dot)} />
      <span>{c.label}</span>
      {status === "reconnecting" && reconnectAttempts > 0 && (
        <span className="text-muted-foreground">· attempt {reconnectAttempts}</span>
      )}
    </div>
  );
}
