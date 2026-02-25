import { For, Switch, Match, Accessor } from "solid-js";
import { FaSolidSpinner, FaSolidCircleCheck } from "solid-icons/fa";
import type { UIBlock } from "../../signals/chatSignals";
import { ProseText } from "./ProseText";
import LoadingDots from "./LoadingDots";
import { ArkCollapsible } from "../../ark/ArkCollapsible";

interface ToolCallItemProps {
  toolName: string;
  displayName?: string;
  state: "invoked" | "completed" | "error";
  displayMessage?: string;
  error?: string;
}

function ToolCallItem(props: ToolCallItemProps) {
  return (
    <div class="flex flex-col gap-2 mt-2">
      <div class="text-xs text-neutral-500 whitespace-pre-wrap">
        {props.displayMessage ?? props.toolName.replace(/_/g, " ")}
      </div>
      {props.error && (
        <span class="text-xs text-red-400">{props.error}</span>
      )}
    </div>
  );
}

interface UIBlockListProps {
  blocks: UIBlock[];
  showLoading?: boolean;
}

export default function UIBlockList(props: UIBlockListProps) {
  const bottomSpaceForScroll = (block: UIBlock, i: Accessor<number>) => {
    return i() === props.blocks.length - 1 ? "pb-[80%]" : ""
  }

  return (
    <div class="w-full max-w-4xl mx-auto h-full flex flex-col px-4 pb-8 pt-4">
      <For each={props.blocks}>
        {(block, i) => (
          <div
            class={bottomSpaceForScroll(block, i)}
          >
            <div>
              <Switch>
                <Match when={block.role === "user"}>
                  <div class="mt-4 flex justify-start">
                    <div class="bg-neu-800 text-neu-100 px-5 py-2.5 rounded-2xl max-w-md shadow-sm">
                      <p class="text-sm font-medium leading-relaxed">{(block.data as any).content}</p>
                    </div>
                  </div>
                </Match>

                <Match when={block.role === "queued"}>
                  <div class="mt-4 flex justify-start">
                    <div class="text-neu-400 px-5 py-2.5 rounded-2xl max-w-md border-2 border-dashed border-neu-700">
                      <p class="text-sm leading-relaxed">{(block.data as any).content}</p>
                    </div>
                  </div>
                </Match>

                <Match when={block.role === "assistant"}>
                  <div class="mt-4 flex flex-col gap-3">
                    <div>
                      <ProseText content={(block.data as any).content} />
                    </div>
                  </div>
                </Match>

                <Match when={block.role === "reasoning"}>
                  <div class="mt-4 flex flex-col gap-3 pl-4 border-l-4 border-neutral-700 bg-neutral-900/50 py-2">
                    <div class="text-sm text-neutral-400">
                      <ProseText content={(block.data as any).content} />
                    </div>
                  </div>
                </Match>

                <Match when={block.role === "tool"}>
                  <div class="mt-4">
                    <ArkCollapsible
                      toggle={
                        <div class="flex items-center gap-2 text-sm">
                          <Switch>
                            <Match when={(block.data as any).state === "invoked"}>
                              <FaSolidSpinner class="animate-spin text-neu-500" size={14} />
                            </Match>
                            <Match when={(block.data as any).state === "completed"}>
                              <FaSolidCircleCheck size={14} class="text-neu-500" />
                            </Match>
                          </Switch>
                          <span class="text-neu-300">{(block.data as any).displayName ?? (block.data as any).toolName}</span>
                        </div>
                      }
                      defaultOpen={false}
                    >
                      <ToolCallItem
                        toolName={(block.data as any).toolName}
                        displayName={(block.data as any).displayName}
                        state={(block.data as any).state}
                        displayMessage={(block.data as any).displayMessage}
                        error={(block.data as any).error}
                      />
                    </ArkCollapsible>
                  </div>
                </Match>

                <Match when={block.role === "system"}>
                  <div class="text-neu-500 text-sm">
                    {(block.data as any).content}
                  </div>
                </Match>

                <Match when={block.role === "error"}>
                  <div class="mt-4 px-4 py-3 rounded-xl bg-red-950/40 border border-red-800/50 text-red-300 text-sm">
                    {(block.data as any).message}
                  </div>
                </Match>
              </Switch>

              {props.showLoading && i() === props.blocks.length - 1 && (
                <div class="mt-4">
                  <LoadingDots />
                </div>
              )}
            </div>
          </div>
        )}
      </For>
    </div>
  );
}
