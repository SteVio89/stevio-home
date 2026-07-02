import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';
import { useNavigate } from 'react-router-dom';
import { getMe, logout as apiLogout } from '../api/client';
import { useSiteConfig } from './SiteConfigContext';
import { localeFromPath } from '../i18n/useT';

interface AuthState {
  loading: boolean;
  loggedIn: boolean;
  email: string | null;
  isAdmin: boolean;
}

interface AuthContextValue extends AuthState {
  logout: () => Promise<void>;
  refreshAuth: () => Promise<void>;
}

const noop = async () => {};

const AuthContext = createContext<AuthContextValue>({
  loading: true,
  loggedIn: false,
  email: null,
  isAdmin: false,
  logout: noop,
  refreshAuth: noop,
});

export function AuthProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<AuthState>({
    loading: true,
    loggedIn: false,
    email: null,
    isAdmin: false,
  });
  const navigate = useNavigate();
  const { locales } = useSiteConfig();
  const supportedCodes = useMemo(() => locales.map(l => l.code), [locales]);

  const probe = useCallback(async () => {
    try {
      const me = await getMe();
      const storedEmail = localStorage.getItem('user_email');
      setState({ loading: false, loggedIn: true, email: storedEmail, isAdmin: me.is_admin });
    } catch {
      setState({ loading: false, loggedIn: false, email: null, isAdmin: false });
    }
  }, []);

  useEffect(() => {
    probe();
  }, [probe]);

  const logout = useCallback(async () => {
    try {
      await apiLogout();
    } catch {
      // ignore — session may already be gone
    }
    localStorage.removeItem('user_email');
    setState({ loading: false, loggedIn: false, email: null, isAdmin: false });
    // Detect current locale from URL, default to 'de'
    const locale = localeFromPath(window.location.pathname, supportedCodes);
    navigate(`/${locale}/login`);
  }, [navigate, supportedCodes]);

  return (
    <AuthContext.Provider value={{ ...state, logout, refreshAuth: probe }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  return useContext(AuthContext);
}
