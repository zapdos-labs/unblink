import { JSX, onMount, Show } from "solid-js";
import { authState, initAuth, authScreen, setAuthScreen } from "../signals/authSignals";
import LogInScreen from "../auth/LogInScreen";
import CreateAccountScreen from "../auth/CreateAccountScreen";

interface AuthenticatedProps {
  children: JSX.Element;
}

export function Authenticated(props: AuthenticatedProps) {
  onMount(() => {
    const state = authState();
    console.log("[Authenticated] onMount state:", state);
    if (!state.isAuthenticated) {
      console.log("[Authenticated] Not authenticated, initializing auth...");
      initAuth();
    } else {
      console.log("[Authenticated] Already authenticated");
    }
  });

  return (
    <Show
      fallback={<div class="flex h-screen items-center justify-center text-white">Loading...</div>}
      when={authState().isAuthenticated && !authState().isLoading}>
      <Show when={authScreen() === "login"}>
        <LogInScreen
          onSwitchToRegister={() => setAuthScreen("create-account")}
          onSuccess={() => setAuthScreen(null)}
        />
      </Show>
      <Show when={authScreen() === "create-account"}>
        <CreateAccountScreen
          onSwitchToLogin={() => setAuthScreen("login")}
          onSuccess={() => setAuthScreen(null)}
        />
      </Show>
      <Show when={!authScreen()}>
        {props.children}
      </Show>
    </Show>
  );
}
