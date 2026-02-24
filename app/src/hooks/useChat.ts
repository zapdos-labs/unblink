import { createEffect, createSignal } from "solid-js";
import { chatClient } from "@/src/lib/rpc";
import type { Conversation, UIBlock as ProtoUIBlock } from "@/gen/chat/v1/chat_pb";
import {
  uiBlocks,
  setUIBlocks,
  inputValue,
  setInputValue,
  isLoading,
  setIsLoading,
  activeConversationId,
  setActiveConversationId,
  setChatInputState,
  type UIBlock,
  type UIBlockData,
  type UIRole,
} from "../signals/chatSignals";

// Conversations list
const [conversations, setConversations] = createSignal<Conversation[]>([]);

let abortController: AbortController | null = null;
let firstChunkReceived = false;

// Track UI blocks by ID for replacement (same ID = replace, different ID = new)
const blockMap = new Map<string, UIBlock>();

// Helper: Convert protobuf UIBlock to local UIBlock
function protoToUIBlock(proto: ProtoUIBlock): UIBlock {
  let data: UIBlockData;
  try {
    data = JSON.parse(proto.data);
  } catch (e) {
    console.error("Failed to parse UI block data:", e);
    data = { content: "" };
  }

  // Protobuf timestamps are optional, but server always sets them
  if (!proto.createdAt) {
    throw new Error("UIBlock missing createdAt timestamp");
  }

  return {
    id: proto.id,
    conversationId: proto.conversationId,
    role: proto.role as UIRole,
    data,
    createdAt: Number(proto.createdAt.seconds) * 1000,
  };
}

// Helper function to upsert a block (update if exists, insert if new)
const upsertBlock = (block: UIBlock) => {
  blockMap.set(block.id, block);
  // Return sorted blocks by created_at
  const sorted = Array.from(blockMap.values()).sort((a, b) =>
    a.createdAt - b.createdAt
  );
  setUIBlocks(sorted);
};

export function useChat() {
  const [streamingContent, setStreamingContent] = createSignal("");

  createEffect(() => {
    const _ = activeConversationId();
  })

  const sendMessage = async () => {
    const message = inputValue().trim();
    if (!message || isLoading()) return;

    // Ensure we have an active conversation
    let currentId = activeConversationId();
    if (!currentId) {
      const newConv = await createConversation();
      if (!newConv) return;
      currentId = newConv.id;
      setActiveConversationId(currentId);
      // Clear block map for new conversation
      blockMap.clear();
    }

    setInputValue("");
    setIsLoading(true);
    setStreamingContent("");
    setChatInputState('user_sent');
    firstChunkReceived = false;

    // Set up abort controller for this request
    abortController = new AbortController();

    try {
      const stream = chatClient.sendMessage(
        {
          conversationId: currentId ?? undefined,
          content: message,
          useWebSearch: false,
        },
        { signal: abortController.signal }
      );

      for await (const response of stream) {
        if (response.event.case === "delta") {
          const deltaEvent = response.event.value;
          const blockId = deltaEvent.blockId;
          const delta = deltaEvent.delta;

          console.log("[useChat] Delta event:", { blockId, delta });

          // Find the block by ID and append the delta to its content
          const existingBlock = blockMap.get(blockId);
          if (existingBlock) {
            // Set first chunk state on first delta
            if (!firstChunkReceived && existingBlock.role === "assistant") {
              setChatInputState('first_chunk_arrived');
              firstChunkReceived = true;
            }

            // Append delta to existing content
            const updatedBlock: UIBlock = {
              ...existingBlock,
              data: {
                ...existingBlock.data,
                content: (existingBlock.data as any).content + delta,
              },
            };
            blockMap.set(blockId, updatedBlock);
            setUIBlocks(Array.from(blockMap.values()).sort((a, b) =>
              a.createdAt - b.createdAt
            ));

            // Update streaming content for assistant blocks
            if (existingBlock.role === "assistant") {
              setStreamingContent((prev) => prev + delta);
            }
          } else {
            console.warn("[useChat] Block not found for delta:", blockId);
          }
        } else if (response.event.case === "uiBlock") {
          const uiBlockEvent = response.event.value;
          const block = protoToUIBlock(uiBlockEvent);

          // Upsert the block (same ID = replace, different ID = new)
          upsertBlock(block);
        }
      }
    } catch (error) {
      console.error("Error sending message:", error);
    } finally {
      setIsLoading(false);
      setStreamingContent("");
      setChatInputState('idle');
      abortController = null;
      // Refresh conversations to update lastUpdated time
      await listConversations();
    }
  };

  const createConversation = async (): Promise<Conversation | null> => {
    try {
      const response = await chatClient.createConversation({
        title: "",
      });
      const conv = response.conversation;
      if (conv) {
        // Refresh the conversation list
        await listConversations();
        return conv;
      }
    } catch (error) {
      console.error("Error creating conversation:", error);
    }
    return null;
  };

  const listConversations = async () => {
    try {
      const response = await chatClient.listConversations({
        pageSize: 50,
        pageToken: "",
      });
      setConversations(response.conversations);
    } catch (error) {
      console.error("Error listing conversations:", error);
    }
  };

  const loadConversation = async (conversationId: string) => {
    try {
      // Clear block map for new conversation
      blockMap.clear();

      const response = await chatClient.listUIBlocks({
        conversationId,
      });

      const blocks: UIBlock[] = response.uiBlocks.map(protoToUIBlock);

      // Populate block map and set UI blocks
      blocks.forEach((block) => blockMap.set(block.id, block));
      setUIBlocks(blocks);
      setActiveConversationId(conversationId);
    } catch (error) {
      console.error("Error loading conversation:", error);
    }
  };

  const deleteConversation = async (conversationId: string) => {
    try {
      await chatClient.deleteConversation({ conversationId });
      // If we deleted the active conversation, clear state
      if (activeConversationId() === conversationId) {
        setActiveConversationId(null);
        blockMap.clear();
        setUIBlocks([]);
      }
      await listConversations();
    } catch (error) {
      console.error("Error deleting conversation:", error);
    }
  };

  const stopGeneration = () => {
    if (abortController) {
      abortController.abort();
      abortController = null;
    }

    // Reset all streaming/loading states
    setIsLoading(false);
    setStreamingContent("");
    setChatInputState('idle');
    firstChunkReceived = false;
  };

  const handleSelectConversation = (id: string) => {
    if (id === activeConversationId()) return;

    // Stop any ongoing generation
    if (abortController) {
      abortController.abort();
      abortController = null;
    }

    // Reset all streaming/loading states
    setIsLoading(false);
    setStreamingContent("");
    setChatInputState('idle');
    firstChunkReceived = false;
    setInputValue("");

    // Switch conversation
    setActiveConversationId(id);
    blockMap.clear();
    setUIBlocks([]); // Clear while loading
    loadConversation(id);
  };

  const handleNewChat = () => {
    // Stop any ongoing generation
    if (abortController) {
      abortController.abort();
      abortController = null;
    }

    // Reset all streaming/loading states
    setIsLoading(false);
    setStreamingContent("");
    setChatInputState('idle');
    firstChunkReceived = false;
    setInputValue("");

    // Clear conversation
    setActiveConversationId(null);
    blockMap.clear();
    setUIBlocks([]);
  };

  return {
    conversations,
    sendMessage,
    createConversation,
    listConversations,
    loadConversation,
    deleteConversation,
    stopGeneration,
    handleSelectConversation,
    handleNewChat,
    streamingContent,
  };
}
