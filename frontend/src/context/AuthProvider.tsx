import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react';
import { useNavigate } from 'react-router-dom';
import { getMe, logout as apiLogout } from '../api/client';
import { useSiteConfig } from './SiteConfigContext';
import { localeFromPath } from '../i18n/useT';
import { AuthContext, type AuthState } from './AuthContext';

// Probe the session and map the result to auth state. Returns (never sets) state
// so callers keep setState lexically after the await.
async function fetchAuthState(): Promise<AuthState> {
  try {
    const me = await getMe();
    const storedEmail = localStorage.getItem('user_email');
    return { loading: false, loggedIn: true, email: storedEmail, isAdmin: me.is_admin };
  } catch {
    return { loading: false, loggedIn: false, email: null, isAdmin: false };
  }
}

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

  const refreshAuth = useCallback(async () => {
    setState(await fetchAuthState());
  }, []);

  useEffect(() => {
    (async () => {
      setState(await fetchAuthState());
    })();
  }, []);

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
    <AuthContext.Provider value={{ ...state, logout, refreshAuth }}>
      {children}
    </AuthContext.Provider>
  );
}
