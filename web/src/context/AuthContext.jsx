import { createContext, useCallback, useContext, useEffect, useState } from "react";
import * as api from "../lib/api";

const AuthContext = createContext(null);

export function AuthProvider({ children }) {
  const [user, setUser] = useState(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    api
      .getMe()
      .then((u) => {
        if (!cancelled) setUser(u);
      })
      .catch(() => {
        if (!cancelled) setUser(null);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  // Login only ever resolves with a real, verified user — an unverified
  // account makes the API throw ApiError with body.pendingVerification
  // set instead (see lib/api.js), which the caller (Login.jsx) catches
  // and handles by showing the "check your inbox" screen, not by asking
  // this function to model that state.
  const doLogin = useCallback(async (email, password) => {
    const u = await api.login(email, password);
    setUser(u);
    return u;
  }, []);

  // Never sets `user` — signup no longer starts a session (login requires
  // a verified email, and a fresh signup never is yet). Resolves with
  // {email, pendingVerification: true} for the caller to act on.
  const doSignup = useCallback(async (email, password, firstName, lastName) => {
    return api.signup(email, password, firstName, lastName);
  }, []);

  const doLogout = useCallback(async () => {
    await api.logout().catch(() => {});
    setUser(null);
  }, []);

  // Uploading doesn't return the updated user, so flip hasAvatar locally
  // right after a successful upload instead of round-tripping to
  // /api/auth/me just to learn what we already know happened.
  const doUploadAvatar = useCallback(async (file) => {
    await api.uploadAvatar(file);
    setUser((prev) => (prev ? { ...prev, hasAvatar: true } : prev));
  }, []);

  // Returns the updated user (the API echoes it back) so the caller can
  // show a fresh value immediately, and updates context state the same
  // way so the topbar name updates without a reload.
  const doUpdateProfile = useCallback(async (firstName, lastName) => {
    const u = await api.updateProfile(firstName, lastName);
    setUser(u);
    return u;
  }, []);

  // Doesn't touch `user` — a password change doesn't change anything
  // AuthContext tracks, and staying signed in afterward is the whole
  // point (unlike the emailed-reset-link flow, this one already proved
  // the caller knows the current password).
  const doChangePassword = useCallback(async (currentPassword, newPassword) => {
    await api.changePassword(currentPassword, newPassword);
  }, []);

  return (
    <AuthContext.Provider
      value={{
        user,
        loading,
        login: doLogin,
        signup: doSignup,
        logout: doLogout,
        uploadAvatar: doUploadAvatar,
        updateProfile: doUpdateProfile,
        changePassword: doChangePassword,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used within AuthProvider");
  return ctx;
}
