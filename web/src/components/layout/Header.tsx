import { Link, useNavigate } from "react-router-dom";
import { LogOut, Radio, ShieldCheck } from "lucide-react";

import { Button } from "@/components/ui/button";
import { BellIcon } from "@/components/notifications/BellIcon";
import { ConnectionStatus } from "@/components/notifications/ConnectionStatus";
import { useAppDispatch, useAppSelector } from "@/store";
import { clearSession } from "@/store/authSlice";
import { clearAll } from "@/store/notificationsSlice";
import { setToken } from "@/lib/api";

export function Header() {
  const user = useAppSelector((s) => s.auth.user);
  const dispatch = useAppDispatch();
  const nav = useNavigate();

  const logout = () => {
    setToken(null);
    dispatch(clearSession());
    dispatch(clearAll());
    nav("/login");
  };

  return (
    <header className="sticky top-0 z-40 border-b bg-background/80 backdrop-blur-md">
      <div className="mx-auto flex h-16 w-full max-w-6xl items-center justify-between px-4 sm:px-6 lg:px-8">
        <Link to="/" className="flex items-center gap-2 font-semibold">
          <span className="grid h-8 w-8 place-items-center rounded-lg bg-primary text-primary-foreground shadow-sm">
            <Radio className="h-4 w-4" />
          </span>
          <span className="text-base tracking-tight">RealtimeHub</span>
        </Link>

        <div className="flex items-center gap-3">
          <ConnectionStatus />
          {user?.role === "admin" && (
            <Button variant="outline" size="sm" asChild>
              <Link to="/admin">
                <ShieldCheck className="h-4 w-4" />
                Admin
              </Link>
            </Button>
          )}
          <BellIcon />
          {user && (
            <div className="hidden items-center gap-2 sm:flex">
              <span className="text-sm text-muted-foreground">{user.username}</span>
              <Button variant="ghost" size="icon" onClick={logout} aria-label="Logout">
                <LogOut className="h-4 w-4" />
              </Button>
            </div>
          )}
        </div>
      </div>
    </header>
  );
}
