import { type Component, type JSX, createMemo } from 'solid-js';
import { authState } from '../shared';

interface AuthenticatedProps {
  children: JSX.Element;
}

const Authenticated: Component<AuthenticatedProps> = (props) => {
  const state = createMemo(() => authState());

  return (
    <>
      {createMemo(() => {
        const current = state();
        console.log('[Authenticated] Rendering - isLoading:', current.isLoading, 'isAuthenticated:', current.isAuthenticated);

        if (current.isLoading) {
          console.log('[Authenticated] Still loading auth state...');
          return (
            <div class="flex justify-center items-center h-screen bg-neutral-950">
              <p class="text-neu-500">Loading...</p>
            </div>
          );
        }

        if (!current.isAuthenticated) {
          console.log('[Authenticated] Not authenticated, redirecting...');
          return (
            <div class="flex justify-center items-center h-screen bg-neutral-950">
              <p class="text-neu-500">Loading...</p>
            </div>
          );
        }

        console.log('[Authenticated] User authenticated, rendering children');
        return props.children;
      })}
    </>
  );
};

export default Authenticated;
