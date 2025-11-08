import { createEffect, createSignal, onMount, Show } from "solid-js";
import { fetchSettings, settings, settingsLoaded } from "./shared";
import LogInScreen from "./auth/LogInScreen";

export default function Authed(props: {
    children: any
}) {
    const [isAuthenticated, setIsAuthenticated] = createSignal(false);
    const [isLoading, setIsLoading] = createSignal(true);

    onMount(async () => {
        await fetchSettings(); // Ensure settings are loaded first

        if (settings()['auth_screen_enabled'] !== 'true') {
            setIsAuthenticated(true);
            setIsLoading(false);
            return;
        }

        try {
            const response = await fetch("/auth/me");
            if (response.ok) {
                setIsAuthenticated(true);
            } else {
                setIsAuthenticated(false);
            }
        } catch (error) {
            console.error("Authentication check failed:", error);
            setIsAuthenticated(false);
        } finally {
            setIsLoading(false);
        }
    });

    return (
        <Show when={!isLoading() && settingsLoaded()} fallback={<div class="flex items-center justify-center h-screen text-white">Loading...</div>}>
            <Show when={isAuthenticated()} fallback={<LogInScreen />}>
                {props.children}
            </Show>
        </Show>
    );
}