import { authState, initAuth, logout } from "../signals/authSignals";
import { authClient } from "../lib/rpc";

export function useAuth() {
  const getUser = async () => {
    try {
      const response = await authClient.getUser({});
      if (response.success && response.user) {
        return response.user;
      }
    } catch (error) {
      console.error("[useAuth] Failed to get user:", error);
    }
    return null;
  };

  return {
    user: () => authState().user,
    isInitialized: () => !authState().isLoading,
    isLoading: () => authState().isLoading,
    isAuthenticated: () => authState().isAuthenticated,
    getUser,
    logout,
  };
}
