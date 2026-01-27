import {
  FiChevronLeft,
  FiChevronRight,
  FiMessageCircle,
} from "solid-icons/fi";
import { createSignal, For, Show } from "solid-js";
import { services, activeTab, setActiveTab, type Tab, fetchServices } from "../shared";
import AddServiceButton from "./AddServiceButton";
import ServiceItem from "./ServiceItem";
import ServiceEditSheet from "./ServiceEditSheet";

interface SideBarProps {
  nodeId: string;
}

export default function SideBar(props: SideBarProps) {
  const [collapsed, setCollapsed] = createSignal(false);
  const [detailServiceId, setDetailServiceId] = createSignal<string | null>(null);

  // Check if a service is currently being viewed
  const isServiceActive = (serviceId: string) => {
    const currentTab = activeTab();
    return (
      currentTab.type === "view" &&
      (currentTab as Extract<Tab, { type: "view" }>).serviceId === serviceId
    );
  };

  // Get the service being edited
  const editingService = () => {
    const id = detailServiceId();
    if (!id) return null;
    return services().find(s => s.id === id) ?? null;
  };

  const handleServiceUpdated = async () => {
    await fetchServices(props.nodeId);
  };

  return (
    <>
      <div
        class={`${collapsed() ? "w-20" : "w-80"
          } h-screen select-none transition-all duration-300 border-r border-neu-800`}
      >
        <div class="bg-neu-900 h-full  flex flex-col drop-shadow-2xl">
          {/* Head */}
          <div
            class={`mt-4 flex items-center ${collapsed() ? "justify-center" : "space-x-3 mx-4"
              } mb-8`}
          >
            <img src="/logo.svg" class="w-12 h-12" />
            <Show when={!collapsed()}>
              <div class="flex-1 font-montserrat font-semibold text-white text-3xl mt-2 leading-none">
                Unblink
              </div>
            </Show>
          </div>

          <div class={`${collapsed() ? "mx-2" : "mx-4"} space-y-1 mb-4`}>
            {/* Chat Tab */}
            <button
              onClick={() => setActiveTab({ type: "chat" })}
              data-active={activeTab().type === "chat"}
              class={`w-full flex items-center ${collapsed() ? "justify-center px-2" : "space-x-3 px-4"
                } py-2 rounded-xl text-neu-400 hover:bg-neu-800 data-[active=true]:bg-neu-800 data-[active=true]:text-white`}
              title={collapsed() ? "Chat" : undefined}
            >
              <FiMessageCircle class="w-4 h-4 flex-shrink-0" />
              <Show when={!collapsed()}>
                <div>Chat</div>
              </Show>
            </button>
          </div>

          {/* Divider */}
          <div class={`${collapsed() ? "mx-2" : "mx-4"} mb-4 h-px bg-neu-800`} />

          {/* Services Section */}
          <div class="flex-1 space-y-2 overflow-y-auto">
            {/* Services Header - only show when expanded */}
            {/* <Show when={!collapsed()}>
              <div class="flex items-center space-x-2 mx-4">
                <div class="text-sm font-medium text-neu-500">Services</div>
              </div>
            </Show> */}

            {/* Add Service Button - only show when expanded */}
            <Show when={!collapsed()}>
              <div class="mx-4 mb-2">
                <AddServiceButton nodeId={props.nodeId} />
              </div>
            </Show>

            <div class={`${collapsed() ? "mx-2" : "mx-4"} space-y-1 mb-4`}>
              <Show
                when={services().length > 0}
                fallback={
                  <Show when={!collapsed()}>
                    <div class="text-sm text-neu-500 p-4">
                      No services available
                    </div>
                  </Show>
                }
              >
                <div class="space-y-1">
                  <For each={services()}>
                    {(service) => (
                      <ServiceItem
                        id={service.id}
                        nodeId={service.nodeId}
                        name={service.name || service.id}
                        isActive={isServiceActive(service.id)}
                        collapsed={collapsed()}
                        onClick={() => {
                          setActiveTab({
                            type: "view",
                            nodeId: service.nodeId,
                            serviceId: service.id,
                            name: service.name || service.id,
                          });
                        }}
                        onMenuSelect={(id) => {
                          if (id === "edit") {
                            setDetailServiceId(service.id);
                          }
                        }}
                      />
                    )}
                  </For>
                </div>
              </Show>
            </div>

          </div>

          {/* Collapse Toggle */}
          <div
            class={`flex-none ${collapsed() ? "mx-2" : "mx-4"} py-4`}
          >
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

      {/* Service Edit Sheet */}
      <Show when={editingService()}>
        {(service) => (
          <ServiceEditSheet
            service={service()}
            open={!!detailServiceId()}
            onOpenChange={(open: boolean) => setDetailServiceId(open ? detailServiceId()! : null)}
            onServiceUpdated={handleServiceUpdated}
          />
        )}
      </Show>
    </>
  );
}
