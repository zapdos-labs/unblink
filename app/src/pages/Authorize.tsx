import { createSignal, onMount, type Component } from 'solid-js';
import { Dynamic } from 'solid-js/web';
import { FaSolidCircleCheck } from 'solid-icons/fa';
import { authState, initAuth } from '../shared';

const LoadingView: Component = () => (
  <div class="flex justify-center items-center h-screen bg-neu-950">
    <p class="text-neu-500">Loading...</p>
  </div>
);

const RedirectToLogin: Component = () => {
  onMount(() => {
    const params = new URLSearchParams(window.location.search);
    const node = params.get('node');
    const dest = node ? `/authorize?node=${node}` : '/authorize';
    window.location.href = `/login?redirect=${encodeURIComponent(dest)}`;
  });
  return null;
};

const SuccessView: Component = () => (
  <div class="flex items-center justify-center h-screen bg-neu-950">
    <div class="bg-neu-900 p-8 rounded-lg shadow-lg w-96 space-y-4 text-center">
      <div class="flex justify-center text-green-400">
        <FaSolidCircleCheck class="w-16 h-16" />
      </div>
      <h2 class="text-2xl font-semibold text-white">Node Authorized!</h2>
      <p class="text-sm text-neu-400">Redirecting to dashboard...</p>
    </div>
  </div>
);

const AuthorizeForm: Component<{
  nodeId: () => string
  loading: () => boolean
  error: () => string
  handleAuthorize: (e: Event) => void
  userEmail: () => string | undefined
}> = (props) => (
  <div class="flex items-center justify-center h-screen bg-neu-950">
    <form onSubmit={props.handleAuthorize} class="bg-neu-900 p-8 rounded-lg shadow-lg w-96 border border-neu-800 space-y-4">
      <div class="space-y-2">
        <div class="flex justify-center">
          <img src="/logo.svg" class="w-18 h-18" alt="Unblink Logo" />
        </div>
        <h2 class="text-2xl font-semibold text-white text-center">Authorize Node</h2>
        <p class="text-xs text-neu-500 text-center break-all font-mono leading-tight">Node ID:<br />{props.nodeId()}</p>
      </div>

      {props.error() && (
        <div class="bg-red-900/30 text-red-400 border border-red-800 p-3 rounded-lg text-sm">
          {props.error()}
        </div>
      )}

      <div class="pt-4">
        <button
          type="submit"
          disabled={props.loading()}
          class="w-full btn-primary disabled:opacity-50 disabled:cursor-not-allowed"
        >
          {props.loading() ? 'Authorizing...' : 'Authorize'}
        </button>
      </div>

      <div class="text-center text-xs text-neu-500 pt-2">
        Logged in as {props.userEmail()}
      </div>
    </form>
  </div>
);

const Authorize: Component = () => {
  const [nodeId, setNodeId] = createSignal('');
  const [loading, setLoading] = createSignal(false);
  const [error, setError] = createSignal('');
  const [success, setSuccess] = createSignal(false);

  onMount(async () => {
    await initAuth();

    const params = new URLSearchParams(window.location.search);
    const node = params.get('node');

    if (!node) {
      setError('Missing node parameter');
      return;
    }

    setNodeId(node);
  });

  const handleAuthorize = async (e: Event) => {
    e.preventDefault();
    setError('');
    setLoading(true);

    try {
      const response = await fetch('/relay/api/authorize', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        credentials: 'include',
        body: JSON.stringify({
          node_id: nodeId(),
        }),
      });

      const data = await response.json();

      if (response.ok && data.success) {
        setSuccess(true);
        setTimeout(() => {
          window.location.href = '/';
        }, 2000);
      } else {
        setError(data.message || 'Authorization failed');
        setLoading(false);
      }
    } catch (err) {
      setError('Network error. Please try again.');
      setLoading(false);
    }
  };

  const auth = authState;

  const view = () => {
    if (auth().isLoading) return LoadingView;
    if (!auth().isAuthenticated) return RedirectToLogin;
    if (success()) return SuccessView;
    return () => AuthorizeForm({
      nodeId,
      loading,
      error,
      handleAuthorize,
      userEmail: () => auth().user?.email,
    });
  };

  return <Dynamic component={view()} />;
};

export default Authorize;
