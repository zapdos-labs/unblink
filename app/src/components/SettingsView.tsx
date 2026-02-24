import { FiUser, FiCpu } from "solid-icons/fi";
import { createEffect, createSignal, Show } from "solid-js";
import { authState, setAuthScreen } from "../signals/authSignals";
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
        title: "Node made private successfully",
        type: "success",
      });
    } catch (error) {
      console.error("Failed to make node private:", error);
      toaster.create({
        title: "Failed to make node private",
        description: error instanceof Error ? error.message : "Unknown error",
        type: "error",
      });
    } finally {
      setIsClaiming(false);
    }
  };

  createEffect(() => {
    console.log('user:', user());
  })

  return (
    <div class="h-full flex items-center justify-center bg-neu-900">
      <div class="w-full max-w-md px-8">
        {/* User Info Section */}
        <div class="space-y-6">

          {/* User Card */}
          <div class="bg-neu-900 border border-neu-800 rounded-2xl p-6">
            <div class="flex items-center gap-4">
              <div class="flex-shrink-0">
                <div class="w-10 h-10 rounded-full bg-neu-800 flex items-center justify-center">
                  <FiUser class="w-5 h-5 text-neu-300" />
                </div>
              </div>
              <div class="flex-1">
                <div class="text-sm text-neu-400">Connected as</div>
                <div class="text-white mt-0.5">
                  {user()?.profile?.name || (user()?.isGuest ? "Guest" : "User")}
                </div>
                <Show when={user()?.email}>
                  <div class="text-sm text-neu-500 truncate">
                    {user()?.email}
                  </div>
                </Show>
              </div>
            </div>
          </div>

          {/* Claim Node Section */}
          <Show when={isClaimed() === false}>
            <div class="bg-neu-900 border border-neu-800 rounded-2xl p-6">
              <div class="text-center space-y-4">
                <div>
                  <div class="text-white mt-1">
                    Node is currently public
                  </div>
                  <div class="text-sm text-neu-400 mt-1">
                    {user()?.isGuest
                      ? "Anyone with the URL can view this node. Create an account or sign in to make it private."
                      : "Make this node private to your account."}
                  </div>
                </div>

                {/* Show Register/Login for guests, Make private for registered users */}
                <Show when={user()?.isGuest}>
                  <div class="flex gap-3">
                    <button
                      onClick={() => setAuthScreen("create-account")}
                      class="flex-1 px-4 py-2 rounded-xl border border-neu-750 bg-neu-800 hover:bg-neu-850 flex items-center justify-center space-x-2"
                    >
                      <span class="text-white">Create Account</span>
                    </button>
                    <button
                      onClick={() => setAuthScreen("login")}
                      class="flex-1 px-4 py-2 rounded-xl border border-neu-750 bg-neu-800 hover:bg-neu-850 flex items-center justify-center space-x-2"
                    >
                      <span class="text-white">Login</span>
                    </button>
                  </div>
                </Show>

                <Show when={!user()?.isGuest}>
                  <button
                    onClick={handleClaimNode}
                    disabled={isClaiming()}
                    class="w-full px-4 py-2 rounded-xl border border-neu-750 bg-neu-800 hover:bg-neu-850 disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center space-x-2"
                  >
                    <span class="text-white">
                      {isClaiming() ? "Making private..." : "Make private"}
                    </span>
                  </button>
                </Show>
              </div>
            </div>
          </Show>



          {/* Node ID Card */}
          <div class="bg-neu-900 border border-neu-800 rounded-2xl p-6">
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

          {/* Node Private Section */}
          <Show when={isClaimed() === true}>
            <div class="bg-neu-900 border border-neu-800 rounded-2xl p-6">
              <div class="text-center space-y-4">
                <div>
                  <div class="text-white mt-1">
                    Node is private
                  </div>
                  <div class="text-sm text-neu-400 mt-1">
                    This node is only accessible to you
                  </div>
                </div>
              </div>
            </div>
          </Show>


        </div>
      </div>
    </div>
  );
}
