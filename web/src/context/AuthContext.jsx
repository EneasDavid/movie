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

  const doLogin = useCallback(async (email, password) => {
    const u = await api.login(email, password);
    setUser(u);
    return u;
  }, []);

  const doSignup = useCallback(async (email, password, firstName, lastName) => {
    const u = await api.signup(email, password, firstName, lastName);
    setUser(u);
    return u;
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

  return (
    <AuthContext.Provider
      value={{ user, loading, login: doLogin, signup: doSignup, logout: doLogout, uploadAvatar: doUploadAvatar }}
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
