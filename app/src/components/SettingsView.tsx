import { FiUser, FiCpu } from "solid-icons/fi";
import { createEffect, createSignal, Show } from "solid-js";
import { authState } from "../signals/authSignals";
import { serviceClient } from "../lib/rpc";
import { toaster } from "../ark/ArkToast";

interface SettingsViewProps {
  nodeId: string;
}

export default function SettingsView(props: SettingsViewProps) {
  const user = () => authState().user;
  const [isClaimed, setIsClaimed] = createSignal<boolean | null>(null);
  const [isClaiming, setIsClaiming] = createSignal(false);

  createEffect(async () => {
    try {
      const res = await serviceClient.listUserNodes({});
      const claimed = res.nodeIds?.includes(props.nodeId) ?? false;
      setIsClaimed(claimed);
    } catch (error) {
      console.error("Failed to check node ownership:", error);
    }
  });

  const handleClaimNode = async () => {
    setIsClaiming(true);
    try {
      await serviceClient.associateUserNode({
        nodeId: props.nodeId,
      });
      setIsClaimed(true);
      toaster.create({
        title: "Node claimed successfully",
        type: "success",
      });
    } catch (error) {
      console.error("Failed to claim node:", error);
      toaster.create({
        title: "Failed to claim node",
        description: error instanceof Error ? error.message : "Unknown error",
        type: "error",
      });
    } finally {
      setIsClaiming(false);
    }
  };

  return (
    <div class="h-full flex items-center justify-center bg-neu-900">
      <div class="w-full max-w-md px-8">
        {/* User Info Section */}
        <div class="space-y-6">

          {/* User Card */}
          <div class="bg-neu-900/50 border border-neu-800 rounded-2xl p-6">
            <div class="flex items-center gap-4">
              <div class="flex-shrink-0">
                <div class="w-10 h-10 rounded-full bg-neu-800 flex items-center justify-center">
                  <FiUser class="w-5 h-5 text-neu-300" />
                </div>
              </div>
              <div class="flex-1">
                <div class="text-sm text-neu-400">Connected as</div>
                <div class="text-white/90 mt-0.5">
                  {user()?.isGuest ? "Guest" : "User"}
                </div>
              </div>
            </div>
          </div>

          {/* Claim Node Section */}
          <Show when={isClaimed() === false}>
            <div class="bg-neu-900/50 border border-neu-800 rounded-2xl p-6">
              <div class="text-center space-y-4">
                <div>
                  <div class="text-white mt-1">
                    This node is currently public
                  </div>
                  <div class="text-sm text-neu-400 mt-1">
                    Sign in to your account and make it private
                  </div>
                </div>
                <button
                  onClick={handleClaimNode}
                  disabled={isClaiming()}
                  class="w-full px-4 py-2 rounded-xl border border-neu-750 bg-neu-800 hover:bg-neu-850 disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center space-x-2"
                >
                  <span class="text-white/90">
                    {isClaiming() ? "Making private..." : "Make private"}
                  </span>
                </button>
              </div>
            </div>
          </Show>

          {/* Node ID Card */}
          <div class="bg-neu-900/50 border border-neu-800 rounded-2xl p-6">
            <div class="flex items-start gap-4">
              <div class="flex-shrink-0 mt-1">
                <div class="w-10 h-10 rounded-full bg-neu-800 flex items-center justify-center">
                  <FiCpu class="w-5 h-5 text-neu-400" />
                </div>
              </div>
              <div class="flex-1 min-w-0">
                <div class="text-xs text-neu-500 uppercase tracking-wider mb-1">Node ID</div>
                <div class="text-sm text-white/80 font-mono truncate">
                  {props.nodeId}
                </div>
              </div>
            </div>
          </div>


        </div>
      </div>
    </div>
  );
}
