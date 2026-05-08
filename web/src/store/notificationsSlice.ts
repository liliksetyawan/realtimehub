import { createSlice, type PayloadAction } from "@reduxjs/toolkit";
import type { Notification } from "@/lib/types";

interface NotificationsState {
  items: Notification[]; // newest first
  unreadCount: number;
  // Last seq known for the current user. Used to send `resume` after a
  // reconnect and ignore re-deliveries for already-rendered seqs.
  lastSeenSeq: number;
}

const initialState: NotificationsState = {
  items: [],
  unreadCount: 0,
  lastSeenSeq: 0,
};

const slice = createSlice({
  name: "notifications",
  initialState,
  reducers: {
    setInitial(state, a: PayloadAction<{ items: Notification[]; unreadCount: number }>) {
      state.items = a.payload.items;
      state.unreadCount = a.payload.unreadCount;
      state.lastSeenSeq = state.items.reduce((max, n) => (n.seq > max ? n.seq : max), 0);
    },
    receive(state, a: PayloadAction<Notification>) {
      const n = a.payload;
      // Dedup by seq (re-delivery from resume can overlap).
      if (state.items.some((x) => x.id === n.id || x.seq === n.seq)) return;
      state.items = [n, ...state.items].slice(0, 200);
      if (!n.read_at) state.unreadCount += 1;
      if (n.seq > state.lastSeenSeq) state.lastSeenSeq = n.seq;
    },
    markRead(state, a: PayloadAction<string>) {
      const n = state.items.find((x) => x.id === a.payload);
      if (n && !n.read_at) {
        n.read_at = new Date().toISOString();
        if (state.unreadCount > 0) state.unreadCount -= 1;
      }
    },
    clearAll(state) {
      state.items = [];
      state.unreadCount = 0;
      state.lastSeenSeq = 0;
    },
  },
});

export const { setInitial, receive, markRead, clearAll } = slice.actions;
export default slice.reducer;
