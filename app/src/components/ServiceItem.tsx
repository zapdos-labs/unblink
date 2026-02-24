import { FiVideo, FiMoreVertical, FiEdit2 } from "solid-icons/fi";
import { Show } from "solid-js";
import { ArkMenu } from "../ark/ArkMenu";

interface ServiceItemProps {
  id: string;
  nodeId: string;
  name: string;
  isActive: boolean;
  collapsed: boolean;
  onClick: () => void;
  onMenuSelect: (id: string) => void;
}

export default function ServiceItem(props: ServiceItemProps) {
  return (
    <div
      data-active={props.isActive}
      class={`w-full flex items-center ${props.collapsed ? "justify-center px-2" : "justify-between px-4"
        } py-2 rounded-xl text-neu-400 hover:bg-neu-800 data-[active=true]:bg-neu-800 data-[active=true]:text-white group cursor-pointer`}
      onClick={(e) => {
        // Don't trigger if clicking on the menu trigger
        if (!(e.target as HTMLElement).closest('[data-part="trigger"]')) {
          props.onClick();
        }
      }}
    >
      <div class="flex items-center space-x-3" title={props.collapsed ? props.name : undefined}>
        <FiVideo class="w-4 h-4 flex-shrink-0" />
        <Show when={!props.collapsed}>
          <div class="text-sm line-clamp-1 break-all">
            {props.name}
          </div>
        </Show>
      </div>
      <Show when={!props.collapsed}>
        <ArkMenu
          items={() => [
            { id: "edit", title: "Edit", icon: <FiEdit2 class="w-4 h-4" /> }
          ]}
          class="group-hover:opacity-100 opacity-0 p-2 border border-neu-750 rounded-lg text-neu-400 hover:bg-neu-750 hover:border-neu-700 hover:text-white transition-colors"
          triggerIcon={<FiMoreVertical class="w-4 h-4" />}
          onSelect={props.onMenuSelect}
          itemRender={(item) => (
            <>
              {item.icon}
              <span>{item.title}</span>
            </>
          )}
        />
      </Show>
    </div>
  );
}
