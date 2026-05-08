import { createSlice, type PayloadAction } from "@reduxjs/toolkit";
import type { User } from "@/lib/types";

interface AuthState {
  user: User | null;
  token: string | null;
}

const initialState: AuthState = {
  user: null,
  token: null,
};

const slice = createSlice({
  name: "auth",
  initialState,
  reducers: {
    setSession(state, a: PayloadAction<{ user: User; token: string }>) {
      state.user = a.payload.user;
      state.token = a.payload.token;
    },
    clearSession(state) {
      state.user = null;
      state.token = null;
    },
  },
});

export const { setSession, clearSession } = slice.actions;
export default slice.reducer;
