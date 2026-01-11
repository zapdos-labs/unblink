import {
  FiGrid,
  FiSearch,
  FiFilm,
  FiEye,
  FiSettings,
  FiChevronLeft,
  FiChevronRight,
  FiVideo,
} from "solid-icons/fi";
import { VsBug } from "solid-icons/vs";
import { createSignal, For, onMount, Show } from "solid-js";
import logoSVG from "../assets/logo.svg";
import {
  fetchNodes,
  nodes,
  nodesLoading,
  setTab,
  tab,
  type Tab,
  type NodeService,
  type SimpleTabType,
} from "../shared";
import UserMenu from "./UserMenu";

export default function SideBar() {
  onMount(fetchNodes);
  const [collapsed, setCollapsed] = createSignal(false);

  // Flatten all services from all nodes into a single list
  const allServices = () => {
    const services: Array<{ service: NodeService; nodeId: string }> = [];
    for (const node of nodes()) {
      for (const service of node.services) {
        services.push({ service, nodeId: node.id });
      }
    }
    return services;
  };

  // Check if a service is currently being viewed
  const isServiceActive = (serviceId: string) => {
    const currentTab = tab();
    return (
      currentTab.type === "view" &&
      (currentTab as Extract<Tab, { type: "view" }>).serviceId === serviceId
    );
  };

  const tabs: Array<{
    type: SimpleTabType;
    name: string;
    icon: typeof FiGrid;
  }> = [
      {
        type: "home",
        name: "Home",
        icon: FiGrid,
      },
      {
        type: "agents",
        name: "Agents",
        icon: FiEye,
      },
      {
        type: "search",
        name: "Search",
        icon: FiSearch,
      },
      {
        type: "moments",
        name: "Moments",
        icon: FiFilm,
      },
    ];

  return (
    <div
      class={`${collapsed() ? "w-20" : "w-80"
        } h-screen pl-2 py-2 select-none transition-all duration-300`}
    >
      <div class="bg-neu-900 h-full rounded-2xl border border-neu-800 flex flex-col drop-shadow-2xl">
        {/* Head */}
        <div
          class={`mt-4 flex items-center ${collapsed() ? "justify-center" : "space-x-3 mx-4"
            } mb-8`}
        >
          <img src={logoSVG} class="w-12 h-12" />
          <Show when={!collapsed()}>
            <div class="flex-1 font-nunito font-medium text-white text-3xl mt-2 leading-none">
              Unblink
            </div>
          </Show>
        </div>

        <div class={`${collapsed() ? "mx-2" : "mx-4"} space-y-1 mb-4`}>
          <For each={tabs}>
            {(_tab) => (
              <button
                onClick={() => setTab({ type: _tab.type })}
                data-active={tab().type === _tab.type}
                class={`w-full flex items-center ${collapsed() ? "justify-center px-2" : "space-x-3 px-4"
                  } py-2 rounded-xl text-neu-400 hover:bg-neu-800 data-[active=true]:bg-neu-800 data-[active=true]:text-white`}
                title={collapsed() ? _tab.name : undefined}
              >
                <_tab.icon class="w-4 h-4 flex-shrink-0" />
                <Show when={!collapsed()}>
                  <div>{_tab.name}</div>
                </Show>
              </button>
            )}
          </For>
        </div>

        <Show when={!collapsed()}>
          <div class="flex-1 space-y-2 overflow-y-auto">
            <div class="flex items-center space-x-2 mx-4">
              <FiVideo size={16} class="text-neu-500" />
              <div class="font-sm font-medium text-neu-500">Services</div>
            </div>
            <Show
              when={!nodesLoading()}
              fallback={
                <div class="text-sm text-neu-500 p-4">Loading services...</div>
              }
            >
              <Show
                when={allServices().length > 0}
                fallback={
                  <div class="text-sm text-neu-500 p-4">
                    No services available
                  </div>
                }
              >
                <div class="space-y-1">
                  <For each={allServices()}>
                    {({ service, nodeId }) => (
                      <div
                        onClick={() => {
                          setTab({
                            type: "view",
                            nodeId,
                            serviceId: service.id,
                            name: service.name || service.id,
                          });
                        }}
                        data-active={isServiceActive(service.id)}
                        class="cursor-pointer px-3 py-2 mx-2 rounded-lg hover:bg-neutral-800 flex items-center text-neutral-400 hover:text-white data-[active=true]:bg-neu-800 data-[active=true]:text-white"
                      >
                        <div class="text-sm line-clamp-1 break-all">
                          {service.name || service.id}
                        </div>
                      </div>
                    )}
                  </For>
                </div>
              </Show>
            </Show>
          </div>
        </Show>

        <Show when={collapsed()}>
          <div class="flex-1" />
        </Show>

        <div
          class={`flex-none ${collapsed() ? "mx-2" : "mx-4"} py-4 space-y-4`}
        >
          {/* User Menu */}
          <UserMenu collapsed={collapsed()} />

          <div
            onClick={() =>
              window.open(
                "https://github.com/unblink/unblink/issues",
                "_blank"
              )
            }
            class={`flex items-center ${collapsed() ? "justify-center" : ""
              } transition hover:text-white text-neu-500 hover:cursor-pointer`}
            title={collapsed() ? "Report Bug" : undefined}
          >
            <VsBug class="w-5 h-5" />
            <Show when={!collapsed()}>
              <div class="ml-2">Report Bug</div>
            </Show>
          </div>

          <div
            onClick={() => setTab({ type: "settings" })}
            data-active={tab().type === "settings"}
            class={`flex items-center ${collapsed() ? "justify-center" : ""
              } transition hover:text-white text-neu-500 hover:cursor-pointer data-[active=true]:text-white`}
            title={collapsed() ? "Settings" : undefined}
          >
            <FiSettings class="w-5 h-5" />
            <Show when={!collapsed()}>
              <div class="ml-2">Settings</div>
            </Show>
          </div>

          {/* Collapse toggle */}
          <div
            onClick={() => setCollapsed(!collapsed())}
            class={`flex items-center ${collapsed() ? "justify-center" : ""
              } transition hover:text-white text-neu-500 hover:cursor-pointer`}
            title={collapsed() ? "Expand" : "Collapse"}
          >
            <Show
              when={collapsed()}
              fallback={<FiChevronLeft class="w-5 h-5" />}
            >
              <FiChevronRight class="w-5 h-5" />
            </Show>
            <Show when={!collapsed()}>
              <div class="ml-2">Hide</div>
            </Show>
          </div>
        </div>
      </div>
    </div>
  );
}
