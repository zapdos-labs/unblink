import { createSignal } from "solid-js";
import { authClient, setToken, getToken, clearToken } from "../lib/rpc";
import type { User } from "../../gen/chat/v1/auth/auth_pb";

export interface AuthState {
  user: User | null;
  isAuthenticated: boolean;
  isLoading: boolean;
}

export const [authState, setAuthState] = createSignal<AuthState>({
  user: null,
  isAuthenticated: false,
  isLoading: true,  // Critical: start as loading
});

export type AuthScreen = "login" | "create-account" | null;
export const [authScreen, setAuthScreen] = createSignal<AuthScreen>(null);

export const initAuth = async () => {
  console.log("[authSignals] Starting auth initialization...");

  const existingToken = getToken();

  if (existingToken) {
    // Validate existing token
    try {
      const response = await authClient.getUser({});
      if (response.success && response.user) {
        setAuthState({
          user: response.user,
          isAuthenticated: true,
          isLoading: false,
        });
        console.log("[authSignals] Existing token valid");
        return;
      }
    } catch (error) {
      console.log("[authSignals] Existing token invalid, clearing");
      clearToken();
    }
  }

  // Create new guest user
  try {
    const response = await authClient.createGuestUser({});
    if (response.success && response.token && response.user) {
      setToken(response.token);
      setAuthState({
        user: response.user,
        isAuthenticated: true,
        isLoading: false,
      });
      console.log("[authSignals] Guest user created");
    }
  } catch (error) {
    console.error("[authSignals] Failed to create guest user:", error);
    setAuthState({
      user: null,
      isAuthenticated: false,
      isLoading: false,
    });
  }
};

export const logout = () => {
  clearToken();
  setAuthState({
    user: null,
    isAuthenticated: false,
    isLoading: false,
  });
};

export const login = async (email: string, password: string) => {
  const response = await authClient.login({ email, password });
  if (response.success && response.token && response.user) {
    setToken(response.token);
    setAuthState({
      user: response.user,
      isAuthenticated: true,
      isLoading: false,
    });
    return true;
  }
  return false;
};

export const createAccount = async (email: string, password: string, name?: string) => {
  const response = await authClient.createAndLinkAccount({
    email,
    password,
    name,
  });
  if (response.success && response.token && response.user) {
    setToken(response.token);
    setAuthState({
      user: response.user,
      isAuthenticated: true,
      isLoading: false,
    });
    return true;
  }
  return false;
};
