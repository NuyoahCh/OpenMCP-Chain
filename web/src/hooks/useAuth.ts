import { useCallback, useEffect, useMemo, useState } from "react";
import {
  authenticate,
  getAuthState,
  isAuthExpired,
  logout as apiLogout,
  subscribeAuth,
  type AuthState
} from "../api";

export interface AuthCredentials {
  username: string;
  password: string;
  scope?: string[];
}

export function useAuth() {
  const [auth, setAuth] = useState<AuthState | null>(() => getAuthState());

  useEffect(() => {
    return subscribeAuth((next) => {
      setAuth(next);
    });
  }, []);

  const login = useCallback(async (credentials: AuthCredentials) => {
    const next = await authenticate(credentials);
    setAuth(next);
    return next;
  }, []);

  const logout = useCallback(() => {
    apiLogout();
    setAuth(null);
  }, []);

  const isExpired = useMemo(() => isAuthExpired(auth), [auth]);

  return {
    auth,
    login,
    logout,
    isExpired
  } as const;
}
