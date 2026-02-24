import { Menu } from '@ark-ui/solid/menu';
import { FiClock } from 'solid-icons/fi';
import { For, createSignal, type JSX } from 'solid-js';
import { Portal } from 'solid-js/web';

export type ArkMenuItem = {
  id: string;
  title: string;
  subtitle?: string;
  icon?: JSX.Element;
};

interface ArkMenuProps {
  items: () => ArkMenuItem[];
  class?: string;
  width?: string;
  triggerIcon?: JSX.Element;
  onSelect?: (id: string) => void;
  activeItemId?: string;
  itemRender: (item: ArkMenuItem) => JSX.Element;
  emptyContent?: JSX.Element;
}

export const ArkMenu = (props: ArkMenuProps) => {
  const [open, setOpen] = createSignal(false);

  const triggerIcon = props.triggerIcon || <FiClock />;
  const menuWidth = props.width || 'min-w-[16rem]';

  return (
    <Menu.Root open={open()} onOpenChange={(details) => setOpen(details.open)}>
      <Menu.Trigger class={`p-2 text-neu-400 hover:text-neu-200 hover:bg-neu-800 rounded-lg transition-colors duration-150 ${props.class || ''}`}>
        {triggerIcon}
      </Menu.Trigger>
      <Portal>
        <Menu.Positioner>
          <Menu.Content class={`bg-neu-850 border border-neu-800 rounded-lg shadow-lg py-1 z-50 focus:outline-none overflow-y-auto max-h-[40vh] ${menuWidth}`}>
            {props.items().length === 0 ? (
              <div class="px-3 py-2 text-sm text-neu-500">
                {props.emptyContent ?? <span>Nothing here</span>}
              </div>
            ) : (
              <For each={props.items()}>
                {(item) => {
                  const isActive = props.activeItemId === item.id;
                  return (
                    <Menu.Item
                      value={item.id}
                      onClick={() => { props.onSelect?.(item.id); setOpen(false); }}
                      class={`flex items-center gap-2 px-3 py-2 text-sm text-neu-400 hover:bg-neu-800 hover:text-white cursor-pointer transition-colors rounded-md mx-1 ${
                        isActive ? 'bg-neu-800 text-white' : ''
                      }`}
                    >
                      {props.itemRender(item)}
                    </Menu.Item>
                  );
                }}
              </For>
            )}
          </Menu.Content>
        </Menu.Positioner>
      </Portal>
    </Menu.Root>
  );
};

export default ArkMenu;
