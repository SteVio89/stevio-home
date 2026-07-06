import { createContext, useContext } from 'react';

export interface AuthState {
  loading: boolean;
  loggedIn: boolean;
  email: string | null;
  isAdmin: boolean;
}

export interface AuthContextValue extends AuthState {
  logout: () => Promise<void>;
  refreshAuth: () => Promise<void>;
}

const noop = async () => {};

export const AuthContext = createContext<AuthContextValue>({
  loading: true,
  loggedIn: false,
  email: null,
  isAdmin: false,
  logout: noop,
  refreshAuth: noop,
});

export function useAuth() {
  return useContext(AuthContext);
}
