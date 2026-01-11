import { Menu } from '@ark-ui/solid/menu';
import { FiLogOut } from 'solid-icons/fi';
import { Show, createSignal, createMemo } from 'solid-js';
import { Portal } from 'solid-js/web';
import { authState, logout } from '../shared';

interface UserMenuProps {
  collapsed?: boolean;
}

const UserMenu = (props: UserMenuProps) => {
  const [open, setOpen] = createSignal(false);

  const userInitial = createMemo(() => {
    const currentUser = authState().user;
    if (!currentUser?.name) return '?';
    return currentUser.name.charAt(0).toUpperCase();
  });

  const userName = createMemo(() => {
    const currentUser = authState().user;
    return currentUser?.name || 'User';
  });

  const userEmail = createMemo(() => {
    const currentUser = authState().user;
    return currentUser?.email || '';
  });

  const handleLogout = async () => {
    await logout();
    setOpen(false);
    window.location.href = '/login';
  };

  return (
    <Menu.Root open={open()} onOpenChange={(details) => setOpen(details.open)}>
      <Menu.Trigger class={`flex items-center ${props.collapsed ? 'justify-center' : 'space-x-3'} hover:bg-neu-800 rounded-lg px-2 py-2 transition-colors cursor-pointer w-full`}>
        <div class="w-10 h-10 rounded-full bg-neu-700 flex items-center justify-center text-white font-semibold flex-shrink-0">
          {userInitial()}
        </div>
        <Show when={!props.collapsed}>
          <div class="flex flex-col text-left">
            <div class="text-sm font-medium text-white">{userName()}</div>
            <div class="text-xs text-neu-500">{userEmail()}</div>
          </div>
        </Show>
      </Menu.Trigger>
      <Portal>
        <Menu.Positioner>
          <Menu.Content class="bg-neu-850 border border-neu-800 rounded-lg shadow-lg py-1 min-w-[180px] z-50 focus:outline-none">
            <div class="px-3 py-2 border-b border-neu-800 mb-1">
              <div class="flex items-center gap-2">
                <div class="w-6 h-6 rounded-full bg-neu-700 flex items-center justify-center text-xs text-white">
                  {userInitial()}
                </div>
                <div class="flex flex-col">
                  <div class="text-sm font-medium text-white">{userName()}</div>
                  <div class="text-xs text-neu-500">Signed in</div>
                </div>
              </div>
            </div>
            <Menu.Item
              value="logout"
              onSelect={handleLogout}
              class="flex items-center gap-2 px-3 py-2 text-sm text-neu-400 hover:bg-neu-800 hover:text-white cursor-pointer transition-colors rounded-md mx-1"
            >
              <FiLogOut class="w-4 h-4" />
              <span>Log out</span>
            </Menu.Item>
          </Menu.Content>
        </Menu.Positioner>
      </Portal>
    </Menu.Root>
  );
};

export default UserMenu;
