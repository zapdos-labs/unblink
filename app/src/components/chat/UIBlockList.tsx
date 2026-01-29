import { For, Switch, Match, Accessor } from "solid-js";
import { FaSolidSpinner, FaSolidCircleCheck } from "solid-icons/fa";
import { BsX } from "solid-icons/bs";
import type { UIBlock } from "../../signals/chatSignals";
import { ProseText } from "./ProseText";
import LoadingDots from "./LoadingDots";
import { ArkCollapsible } from "../../ark/ArkCollapsible";

interface ToolCallItemProps {
  toolName: string;
  state: "invoked" | "completed" | "error";
  displayMessage?: string;
  error?: string;
}

function ToolCallItem(props: ToolCallItemProps) {
  const displayText = () =>
    props.displayMessage ?? props.toolName.replace(/_/g, " ");

  console.log("[ToolCallItem] props:", props);
  console.log("[ToolCallItem] displayText:", displayText());

  return (
    <div class="flex items-center gap-2 text-sm text-white">
      <Switch>
        <Match when={props.state === "invoked"}>
          <FaSolidSpinner class="animate-spin text-neu-500" size={14} />
        </Match>
        <Match when={props.state === "completed"}>
          <FaSolidCircleCheck size={14} class="text-neu-500" />
        </Match>
      </Switch>
      <span>{displayText()}</span>
      {props.error && (
        <span class="text-xs text-red-400 ml-1">{props.error}</span>
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

                <Match when={block.role === "assistant"}>
                  <div class="mt-4">
                    <ProseText content={(block.data as any).content} />
                  </div>
                </Match>

                <Match when={block.role === "reasoning"}>
                  <div class="mt-4 p-3 bg-neu-850 rounded-lg">
                    <ArkCollapsible toggle={<span class="text-neu-300 hover:text-white">Thinking</span>} defaultOpen={true}>
                      <div class="text-sm text-neu-300 whitespace-pre-wrap">
                        {(block.data as any).content}
                      </div>
                    </ArkCollapsible>
                  </div>
                </Match>

                <Match when={block.role === "tool"}>
                  <div class="mt-4">
                    <ToolCallItem
                      toolName={(block.data as any).toolName}
                      state={(block.data as any).state}
                      displayMessage={(block.data as any).displayMessage}
                      error={(block.data as any).error}
                    />
                  </div>
                </Match>

                <Match when={block.role === "system"}>
                  <div class="text-neu-500 text-sm">
                    {(block.data as any).content}
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
