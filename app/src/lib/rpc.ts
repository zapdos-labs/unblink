import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { ChatService } from "@/gen/chat/v1/chat_pb";
import { AuthService } from "@/gen/chat/v1/auth/auth_pb";
import { WebRTCService } from "@/gen/webrtc/v1/webrtc_pb";
import { ServiceService } from "@/gen/service/v1/service_pb";
import { EventService } from "@/gen/service/v1/event_pb";
import { LiveUpdateService } from "@/gen/service/v1/live_update_pb";

declare global {
  interface Window {
    SERVER_META?: {
      servedBy: string;
      version?: string;
      timestamp?: string;
    };
  }
}

// Use /api when served by Go server, otherwise use full URL with /api suffix for dev
const BASE_URL = window.SERVER_META ? '/api' : (import.meta.env.VITE_SERVER_API_URL || `${window.location.protocol}//${window.location.hostname}:${import.meta.env.VITE_SERVER_API_PORT}/api`);
console.log("Using API base URL:", BASE_URL, window.SERVER_META);

// Auth token storage
const TOKEN_KEY = "auth_token";

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY);
}

export function setToken(token: string): void {
  localStorage.setItem(TOKEN_KEY, token);
}

export function clearToken(): void {
  localStorage.removeItem(TOKEN_KEY);
}

// Transport for Connect RPC with auth interceptor
const transport = createConnectTransport({
  baseUrl: BASE_URL,
  interceptors: [
    (next) => async (req) => {
      // Add Authorization header if token exists
      const token = getToken();
      if (token) {
        req.header.set("Authorization", `Bearer ${token}`);
      }
      return next(req);
    },
  ],
});

// Export typed clients
export const chatClient = createClient(ChatService, transport);
export const authClient = createClient(AuthService, transport);
export const webrtcClient = createClient(WebRTCService, transport);
export const serviceClient = createClient(ServiceService, transport);
export const eventClient = createClient(EventService, transport);
export const liveUpdateClient = createClient(LiveUpdateService, transport);
