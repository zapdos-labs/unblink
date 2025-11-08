import { createSignal, onMount, For, type Accessor, type Setter } from "solid-js";
import type { UseSubTab } from "../SettingsContent";
import ArkSwitch from "~/src/ark/ArkSwitch";
import type { User } from "~/shared";

async function getUsers(): Promise<User[]> {
    try {
        const response = await fetch('/users');
        if (!response.ok) {
            console.error('Failed to fetch users');
            return [];
        }
        return response.json();
    } catch (error) {
        console.error('Error fetching users:', error);
        return [];
    }
}

export const useAuthSubTab: UseSubTab = (props) => {
    const [users, setUsers] = createSignal<User[]>([]);

    onMount(async () => {
        const fetchedUsers = await getUsers();
        setUsers(fetchedUsers);
    });

    return {
        comp: () => <div class="space-y-4">
            <div class="space-y-2">
                <div class="font-semibold text-lg">Auth Screen</div>
                <div class="bg-neu-850 border border-neu-800 rounded-lg p-6">
                    <div class="flex items-center justify-between">
                        <ArkSwitch
                            checked={() => props.scratchpad()['auth_screen_enabled'] === 'true'}
                            onCheckedChange={(details) => props.setScratchpad((prev) => ({
                                ...prev,
                                auth_screen_enabled: details.checked ? 'true' : 'false'
                            }))}
                            label="Enable Auth Screen"
                        />
                    </div>
                </div>
            </div>
            <div class="space-y-2">
                <div class="font-semibold text-lg">Users</div>
                <div class="border border-neu-800 rounded-lg bg-neu-850 p-4">
                    <div class="space-y-2 divide-y divide-neu-800">
                        <For each={users()} fallback={<p class="text-neu-400">No users available</p>}>
                            {(user) => (
                                <div class="flex justify-between items-center p-2 ">
                                    <div>
                                        <p class="font-semibold">{user.username}</p>
                                        <p class="text-sm text-neu-400">{user.role}</p>
                                    </div>
                                </div>
                            )}
                        </For>
                    </div>
                </div>
            </div>
        </div>,
        keys: () => [
            {
                name: 'auth_screen_enabled',
                validate: (value) => {
                    if (value !== 'true' && value !== 'false') {
                        return {
                            type: 'error',
                            message: 'Value must be true or false'
                        };
                    }
                    return {
                        type: 'success',
                    };
                }
            }
        ]
    }
}