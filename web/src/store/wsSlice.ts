import { createSlice, type PayloadAction } from "@reduxjs/toolkit";

export type WsStatus =
  | "idle"
  | "connecting"
  | "open"
  | "reconnecting"
  | "closed";

interface WsState {
  status: WsStatus;
  reconnectAttempts: number;
  lastEventAt: number | null; // unix ms
}

const initialState: WsState = {
  status: "idle",
  reconnectAttempts: 0,
  lastEventAt: null,
};

const slice = createSlice({
  name: "ws",
  initialState,
  reducers: {
    setStatus(state, a: PayloadAction<WsStatus>) {
      state.status = a.payload;
      if (a.payload === "open") state.reconnectAttempts = 0;
    },
    bumpReconnect(state) {
      state.reconnectAttempts += 1;
    },
    touch(state) {
      state.lastEventAt = Date.now();
    },
  },
});

export const { setStatus, bumpReconnect, touch } = slice.actions;
export default slice.reducer;
