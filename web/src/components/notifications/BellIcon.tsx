import { Bell } from "lucide-react";
import { motion, AnimatePresence } from "framer-motion";

import { useAppSelector } from "@/store";

export function BellIcon() {
  const unread = useAppSelector((s) => s.notifications.unreadCount);
  return (
    <div className="relative inline-flex h-9 w-9 items-center justify-center rounded-md text-foreground">
      <Bell className="h-5 w-5" />
      <AnimatePresence>
        {unread > 0 && (
          <motion.span
            key={unread}
            initial={{ scale: 0 }}
            animate={{ scale: 1 }}
            exit={{ scale: 0 }}
            transition={{ type: "spring", stiffness: 380, damping: 22 }}
            className="absolute -right-1 -top-1 grid h-4 min-w-[16px] place-items-center rounded-full bg-destructive px-1 text-[10px] font-semibold leading-none text-destructive-foreground"
          >
            {unread > 99 ? "99+" : unread}
          </motion.span>
        )}
      </AnimatePresence>
    </div>
  );
}
