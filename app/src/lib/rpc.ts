import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { ChatService } from "@/gen/chat/v1/chat_pb";
import { AuthService } from "@/gen/chat/v1/auth/auth_pb";
import { WebRTCService } from "@/gen/webrtc/v1/webrtc_pb";
import { ServiceService } from "@/gen/service/v1/service_pb";
import { EventService } from "@/gen/service/v1/event_pb";

const BASE_URL = import.meta.env.SERVER_API_URL ?? `${window.location.protocol}//${window.location.hostname}:8080`;

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
      console.log("[rpc interceptor] Token exists:", !!token);
      console.log("[rpc interceptor] Token value:", token ? token.substring(0, 30) + "..." : null);
      if (token) {
        req.header.set("Authorization", `Bearer ${token}`);
        console.log("[rpc interceptor] Authorization header set");
        console.log("[rpc interceptor] All headers:", Array.from(req.header.entries()));
      } else {
        console.log("[rpc interceptor] No token found in localStorage");
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
