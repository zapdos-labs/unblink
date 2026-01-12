import { createSignal } from "solid-js";
import posthog from "./lib/posthog";


const BASE_URL = import.meta.env.VITE_RELAY_API_URL;
if (!BASE_URL) throw new Error("VITE_RELAY_API_URL is not configured");
export const relay = (path: string) => new URL(path, BASE_URL).href


// JWT token management
const TOKEN_KEY = 'auth_token';

export const setToken = (token: string) => localStorage.setItem(TOKEN_KEY, token);
export const getToken = () => localStorage.getItem(TOKEN_KEY);
export const clearToken = () => localStorage.removeItem(TOKEN_KEY);

// Helper function to make authenticated API requests
// Automatically adds JWT token and relay base URL
export const relayFetch = async (path: string, options?: RequestInit): Promise<Response> => {
  const token = getToken();
  return fetch(relay(path), {
    ...options,
    headers: {
      ...options?.headers,
      ...(token ? { 'Authorization': `Bearer ${token}` } : {}),
    },
    credentials: 'omit', // No longer need cookies
  });
};


// Auth types
export interface User {
  id: number;
  email: string;
  name: string;
}

export interface AuthState {
  user: User | null;
  isAuthenticated: boolean;
  isLoading: boolean;
}

// Auth state
export const [authState, setAuthState] = createSignal<AuthState>({
  user: null,
  isAuthenticated: false,
  isLoading: true,
});

// Login function
export const login = async (email: string, password: string): Promise<{ success: boolean; message?: string; user?: User }> => {
  try {
    const response = await relayFetch('/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email, password }),
    });

    const data = await response.json();

    if (response.ok && data.success) {
      setToken(data.token); // Store JWT token
      setAuthState({
        user: data.user,
        isAuthenticated: true,
        isLoading: false,
      });
      posthog.identify(String(data.user.id));
      posthog.capture('user_logged_in', { email: data.user.email });
      return { success: true, user: data.user };
    }
    posthog.capture('login_failed', { email });
    return { success: false, message: data.message || 'Login failed' };
  } catch (error) {
    console.error('Login error:', error);
    return { success: false, message: 'Network error. Please try again.' };
  }
};

// Register function
export const register = async (email: string, password: string, name: string): Promise<{ success: boolean; message?: string; user?: User }> => {
  try {
    const response = await fetch(relay('/auth/register'), {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email, password, name }),
    });

    const data = await response.json();

    if (response.ok && data.success) {
      posthog.capture('user_registered', { email, name });
      return { success: true, message: 'Registration successful', user: data.user };
    }
    return { success: false, message: data.message || 'Registration failed' };
  } catch (error) {
    console.error('Register error:', error);
    return { success: false, message: 'Network error. Please try again.' };
  }
};

// Logout function
export const logout = async () => {
  clearToken(); // Remove JWT from localStorage
  setAuthState({
    user: null,
    isAuthenticated: false,
    isLoading: false,
  });
  posthog.capture('user_logged_out');
  posthog.reset();
  try {
    await relayFetch('/auth/logout', { method: 'POST' });
  } catch (error) {
    console.error('Logout error:', error);
  }
};

// Initialize auth by checking JWT token with server
export const initAuth = async () => {
  console.log('[initAuth] Starting auth check...');
  try {
    const response = await relayFetch('/auth/me');

    console.log('[initAuth] Response status:', response.status, response.ok);

    if (response.ok) {
      const data = await response.json();
      console.log('[initAuth] Response data:', data);
      if (data.success) {
        setAuthState({
          user: data.user,
          isAuthenticated: true,
          isLoading: false,
        });
        console.log('[initAuth] User authenticated:', data.user);
      } else {
        clearToken();
        setAuthState({
          user: null,
          isAuthenticated: false,
          isLoading: false,
        });
        console.log('[initAuth] Response not successful');
      }
    } else {
      clearToken();
      setAuthState({
        user: null,
        isAuthenticated: false,
        isLoading: false,
      });
      console.log('[initAuth] Response not OK, setting unauthenticated');
    }
  } catch (error) {
    console.error('[initAuth] Auth check failed:', error);
    clearToken();
    setAuthState({
      user: null,
      isAuthenticated: false,
      isLoading: false,
    });
  }
};

export type SimpleTabType = 'home' | 'search' | 'moments' | 'agents' | 'settings';

export type Tab =
  | { type: SimpleTabType }
  | {
    type: 'view';
    nodeId: string;
    serviceId: string;
    name?: string;
  };

export const [tab, setTab] = createSignal<Tab>({ type: 'home' });

// Node services state
export interface NodeService {
  id: string;
  name: string;
  type: string;
  node_id: string;
  addr: string;
  port: number;
  path: string;
}

export interface NodeInfo {
  id: string;
  services: NodeService[];
  status: 'online' | 'offline';
  name?: string;
  lastConnectedAt?: string;
}

export const [nodes, setNodes] = createSignal<NodeInfo[]>([]);
export const [nodesLoading, setNodesLoading] = createSignal(true);

export const fetchNodes = async () => {
  setNodesLoading(true);
  try {
    const response = await relayFetch('/nodes');

    if (response.ok) {
      const data = await response.json(); // Array of {id, status, name, last_connected_at}

      // Fetch services only for online nodes
      const nodesWithServices: NodeInfo[] = await Promise.all(
        data.map(async (nodeData: { id: string; status: string; name?: string; last_connected_at?: string }) => {
          let services: NodeService[] = [];

          // Only fetch services if node is online
          if (nodeData.status === 'online') {
            try {
              const servicesResp = await relayFetch(`/node/${nodeData.id}/services`);
              if (servicesResp.ok) {
                services = await servicesResp.json();
              }
            } catch {
              // Ignore service fetch errors for individual nodes
            }
          }

          return {
            id: nodeData.id,
            status: nodeData.status as 'online' | 'offline',
            name: nodeData.name,
            lastConnectedAt: nodeData.last_connected_at,
            services
          };
        })
      );

      setNodes(nodesWithServices);
    } else if (response.status === 401) {
      // Unauthorized - redirect to login
      window.location.href = '/login';
    } else {
      console.error('Failed to fetch nodes');
      setNodes([]);
    }
  } catch (error) {
    console.error('Error fetching nodes:', error);
    setNodes([]);
  } finally {
    setNodesLoading(false);
  }
};
