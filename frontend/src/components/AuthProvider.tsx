import { createContext, useContext, useEffect, useState, useCallback, type ReactNode } from "react";
import { api, type AuthUser, type AuthStatus } from "@/lib/api";

interface AuthContextValue {
  user: AuthUser | null;
  status: AuthStatus | null;
  loading: boolean;
  login: (username: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
}

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<AuthUser | null>(null);
  const [status, setStatus] = useState<AuthStatus | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    Promise.all([api.auth.status(), api.auth.me()])
      .then(([authStatus, authUser]) => {
        setStatus(authStatus);
        setUser(authUser);
      })
      .catch(() => {
        setStatus({ enabled: false, type: "none" });
        setUser({ authenticated: false });
      })
      .finally(() => setLoading(false));
  }, []);

  const login = useCallback(async (username: string, password: string) => {
    const result = await api.auth.login({ username, password });
    setUser(result);
  }, []);

  const logout = useCallback(async () => {
    await api.auth.logout();
    setUser({ authenticated: false });
  }, []);

  return (
    <AuthContext.Provider value={{ user, status, loading, login, logout }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used within AuthProvider");
  return ctx;
}
