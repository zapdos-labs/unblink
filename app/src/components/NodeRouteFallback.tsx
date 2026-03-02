import { For, Show, createEffect, createSignal } from 'solid-js';

import { serviceClient } from '../lib/rpc';
import { authState } from '../signals/authSignals';
import { ProseText } from './chat/ProseText';

export default function NodeRouteFallback() {
  const [nodeIds, setNodeIds] = createSignal<string[]>([]);
  const [isLoadingNodes, setIsLoadingNodes] = createSignal(false);
  const [nodesError, setNodesError] = createSignal<string | null>(null);
  const isLoggedIn = () => !!authState().user && !authState().user?.isGuest;

  createEffect(() => {
    const user = authState().user;
    if (!user || user.isGuest) {
      setNodeIds([]);
      setNodesError(null);
      return;
    }

    setIsLoadingNodes(true);
    setNodesError(null);

    serviceClient.listUserNodes({})
      .then((res) => {
        setNodeIds(res.nodeIds ?? []);
      })
      .catch((error) => {
        console.error('Failed to load user nodes:', error);
        setNodesError(error instanceof Error ? error.message : String(error));
      })
      .finally(() => {
        setIsLoadingNodes(false);
      });
  });

  const instructionLoggedOut = `# You need your Dashboard URL\n\nRun \`unblink-node\` on your machine.\nAt the end of startup logs, copy **Dashboard URL** (or scan the QR).`;
  const   instructionFindAndClaim = `Run Unblink \`unblink-node\` on that machine.\nAt the end of startup logs, copy **Dashboard URL** (or scan the QR).`;

  return (
    <div class="flex h-full items-center justify-center bg-neu-900 px-6">
      <div class="w-full max-w-2xl rounded-2xl border border-neu-800 bg-neu-900 p-8 space-y-6">
        <Show
          when={isLoggedIn()}
          fallback={<ProseText content={instructionLoggedOut} />}
        >
          <div class="space-y-8">
            <div class="space-y-3">
              <h2 class="text-2xl font-bold">Your nodes</h2>
              <Show when={isLoadingNodes()}>
                <div class="text-neu-500">Loading nodes...</div>
              </Show>

              <Show when={!isLoadingNodes() && !nodesError() && nodeIds().length === 0}>
                <div class="text-neu-500">No nodes linked to this account yet.</div>
              </Show>

              <Show when={!isLoadingNodes() && !!nodesError()}>
                <div class="text-neu-500">Could not load nodes right now.</div>
              </Show>

              <Show when={!isLoadingNodes() && !nodesError() && nodeIds().length > 0}>
                <div class="flex flex-wrap gap-2">
                  <For each={nodeIds()}>
                    {(id) => (
                      <a
                        href={`/node/${id}`}
                        class="rounded-lg border border-neu-750 bg-neu-800 px-3 py-1.5 text-neu-100 hover:bg-neu-750 transition-colors"
                      >
                        {id}
                      </a>
                    )}
                  </For>
                </div>
              </Show>
            </div>

            <div class="space-y-3">
              <h2 class="text-2xl font-bold text-neu-200">Connect other nodes</h2>
              <ProseText content={instructionFindAndClaim} />
            </div>
          </div>
        </Show>
      </div>
    </div>
  );
}
