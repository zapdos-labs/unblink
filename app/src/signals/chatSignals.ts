import { createSignal } from "solid-js";

export type UIRole = "user" | "assistant" | "tool" | "reasoning" | "system" | "queued" | "error";
export type ToolCallState = "invoked" | "completed" | "error";

// Data structures for each UI block type
export interface UserData {
  content: string;
}

export interface ModelData {
  content: string;
}

export interface ReasoningData {
  content: string;
}

export interface ToolData {
  toolName: string;
  name?: string;
  state: ToolCallState;
  displayMessage?: string;
  error?: string;
  content?: string;
}

export interface SystemData {
  content: string;
}

export interface QueuedData {
  content: string;
}

export interface ErrorData {
  message: string;
}

export type UIBlockData = UserData | ModelData | ReasoningData | ToolData | SystemData | QueuedData | ErrorData;

export interface UIBlock {
  id: string;
  conversationId: string;
  role: UIRole;
  data: UIBlockData;
  createdAt: number;
}

// UI blocks in current conversation
export const [uiBlocks, setUIBlocks] = createSignal<UIBlock[]>([]);

// Current input value
export const [inputValue, setInputValue] = createSignal("");

// Loading state for streaming
export const [isLoading, setIsLoading] = createSignal(false);

// Active conversation ID
export const [activeConversationId, setActiveConversationId] = createSignal<string | null>(null);

// Whether we're showing conversation list (history menu)
export const [showHistory, setShowHistory] = createSignal(false);

// Chat input state for scroll effects
export type ChatInputState = 'idle' | 'user_sent' | 'first_chunk_arrived';
export const [chatInputState, setChatInputState] = createSignal<ChatInputState>('idle');

// Textarea focus state
export const [isTextareaFocused, setIsTextareaFocused] = createSignal(false);

// Message queue for when the model is already responding
export const [messageQueue, setMessageQueue] = createSignal<string[]>([]);

// Feed search tag
export const [feedSearchTag, setFeedSearchTag] = createSignal("anime_girl");
