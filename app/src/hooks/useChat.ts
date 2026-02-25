import { createSignal } from "solid-js";
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
  messageQueue,
  setMessageQueue,
  type UIBlock,
  type UIBlockData,
  type UIRole,
} from "@/src/signals/chatSignals";
import { selectedTrait } from "@/src/components/chat/TraitSettings";

// Conversations list
const [conversations, setConversations] = createSignal<Conversation[]>([]);

let abortController: AbortController | null = null;
let firstChunkReceived = false;

// Track UI blocks by ID for replacement (same ID = replace, different ID = new)
const blockMap = new Map<string, UIBlock>();

// Track queued block IDs so we can remove them when processed
const queuedBlockIds: string[] = [];
let queuedBlockCounter = 0;
// Base timestamp for queued blocks to ensure they always appear at the end
const QUEUED_BLOCK_BASE_TIME = Number.MAX_SAFE_INTEGER - 1000000;

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

  // Process next message from queue
  const processQueue = async () => {
    const queue = messageQueue();
    if (queue.length === 0) return;

    const nextMessage = queue[0];
    // Remove from queue
    setMessageQueue(queue.slice(1));

    // Remove the queued UI block
    const queuedBlockId = queuedBlockIds.shift();
    if (queuedBlockId) {
      blockMap.delete(queuedBlockId);
      setUIBlocks(Array.from(blockMap.values()).sort((a, b) =>
        a.createdAt - b.createdAt
      ));
    }

    // Send the queued message
    await sendMessageInternal(nextMessage);
  };

  const sendMessage = async () => {
    const message = inputValue().trim();
    if (!message) return;

    setInputValue("");

    if (isLoading()) {
      // Add to queue if already loading
      setMessageQueue([...messageQueue(), message]);

      // Create a queued UI block with a far-future timestamp to keep it at the end
      const queuedBlockId = `queued-${queuedBlockCounter++}`;
      queuedBlockIds.push(queuedBlockId);
      const queuedBlock: UIBlock = {
        id: queuedBlockId,
        conversationId: activeConversationId() ?? "",
        role: "queued",
        data: { content: message },
        createdAt: QUEUED_BLOCK_BASE_TIME + queuedBlockCounter,
      };
      upsertBlock(queuedBlock);
      return;
    }

    await sendMessageInternal(message);
  };

  const sendMessageInternal = async (message: string) => {
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

    setIsLoading(true);
    setStreamingContent("");
    setChatInputState("user_sent");
    firstChunkReceived = false;

    // Set up abort controller for this request
    abortController = new AbortController();

    try {
      const stream = chatClient.sendMessage(
        {
          conversationId: currentId ?? undefined,
          content: message,
        },
        { signal: abortController.signal }
      );

      for await (const response of stream) {
        if (response.event.case === "delta") {
          const deltaEvent = response.event.value;
          const blockId = deltaEvent.blockId;
          const delta = deltaEvent.delta;

          // Find the block by ID and append the delta to its content
          const existingBlock = blockMap.get(blockId);
          if (existingBlock) {
            // Set first chunk state on first delta
            if (!firstChunkReceived && existingBlock.role === "assistant") {
              setChatInputState("first_chunk_arrived");
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
      // Server unreachable / connection dropped before stream established
      const msg = error instanceof Error ? error.message : String(error);
      upsertBlock({
        id: `error-${Date.now()}`,
        conversationId: currentId ?? "",
        role: "error",
        data: { message: msg },
        createdAt: Date.now(),
      });
    } finally {
      setIsLoading(false);
      setStreamingContent("");
      setChatInputState("idle");
      abortController = null;
      // Refresh conversations to update lastUpdated time
      await listConversations();

      // Process next message from queue
      await processQueue();
    }
  };

  const createConversation = async (): Promise<Conversation | null> => {
    try {
      const response = await chatClient.createConversation({
        title: "",
        trait: selectedTrait(),
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
    setChatInputState("idle");
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
    setChatInputState("idle");
    firstChunkReceived = false;
    setInputValue("");

    // Clear queue and queued blocks
    setMessageQueue([]);
    queuedBlockIds.length = 0;

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
    setChatInputState("idle");
    firstChunkReceived = false;
    setInputValue("");

    // Clear queue and queued blocks
    setMessageQueue([]);
    queuedBlockIds.length = 0;

    // Clear conversation
    setActiveConversationId(null);
    blockMap.clear();
    setUIBlocks([]);
  };

  const removeLatestQueuedMessage = () => {
    const queue = messageQueue();
    if (queue.length === 0) return;

    setMessageQueue(queue.slice(0, -1));

    const queuedBlockId = queuedBlockIds.pop();
    if (queuedBlockId) {
      blockMap.delete(queuedBlockId);
      setUIBlocks(Array.from(blockMap.values()).sort((a, b) =>
        a.createdAt - b.createdAt
      ));
    }
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
    removeLatestQueuedMessage,
    streamingContent,
  };
}
