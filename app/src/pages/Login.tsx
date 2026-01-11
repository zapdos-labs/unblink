import { createSignal, onMount, type Component } from 'solid-js';
import { login, authState, initAuth } from '../shared';

const Login: Component = () => {
  const [email, setEmail] = createSignal('');
  const [password, setPassword] = createSignal('');
  const [error, setError] = createSignal('');
  const [success, setSuccess] = createSignal('');
  const [loading, setLoading] = createSignal(false);

  onMount(async () => {
    await initAuth();

    // If already authenticated, redirect to main page
    if (authState().isAuthenticated) {
      window.location.href = redirectParam() || '/';
      return;
    }

    // Check for message from registration
    const params = new URLSearchParams(window.location.search);
    const msg = params.get('message');
    if (msg) {
      setSuccess(msg);
    }
  });

  const redirectParam = () => {
    const params = new URLSearchParams(window.location.search);
    return params.get('redirect');
  };

  const handleSubmit = async (e: Event) => {
    e.preventDefault();
    setError('');
    setLoading(true);

    const result = await login(email(), password());

    if (result.success) {
      window.location.href = redirectParam() || '/';
    } else {
      setError(result.message || 'Login failed');
      setLoading(false);
    }
  };

  return (
    <div class="flex items-center justify-center h-screen bg-neu-950">
      <form onSubmit={handleSubmit} class="bg-neu-900 p-8 rounded-lg shadow-lg w-96 border border-neu-800 space-y-4">
        <div class="space-y-2">
          <div class="flex justify-center">
            <img src="/logo.svg" class="w-18 h-18" alt="Unblink Logo" />
          </div>
          <h2 class="text-2xl font-semibold text-white text-center">Login to Unblink</h2>
          <h3 class="text-sm text-neu-400 text-center">AI for your camera</h3>
        </div>

        {error() && (
          <div class="bg-red-900/30 text-red-400 border border-red-800 p-3 rounded-lg text-sm">
            {error()}
          </div>
        )}

        {success() && (
          <div class="bg-green-900/30 text-green-400 border border-green-800 p-3 rounded-lg text-sm">
            {success()}
          </div>
        )}

        <div>
          <label for="email" class="text-sm font-medium text-neu-300">
            Email
          </label>
          <input
            type="email"
            id="email"
            placeholder="you@example.com"
            autocomplete="username"
            class="px-3 py-1.5 mt-1 block w-full rounded-lg bg-neu-850 border border-neu-750 text-white focus:outline-none placeholder:text-neu-500"
            value={email()}
            onInput={(e) => setEmail(e.currentTarget.value)}
            required
          />
        </div>

        <div>
          <label for="password" class="text-sm font-medium text-neu-300">
            Password
          </label>
          <input
            type="password"
            id="password"
            placeholder="Enter your password"
            autocomplete="current-password"
            class="px-3 py-1.5 mt-1 block w-full rounded-lg bg-neu-850 border border-neu-750 text-white focus:outline-none placeholder:text-neu-500"
            value={password()}
            onInput={(e) => setPassword(e.currentTarget.value)}
            required
          />
        </div>

        <div class="pt-4">
          <button
            type="submit"
            disabled={loading()}
            class="w-full btn-primary disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {loading() ? 'Signing in...' : 'Sign In'}
          </button>
        </div>

        <div class="flex justify-center text-xs text-neu-500 pt-2">
          Don't have an account?{' '}
          <a
            href={redirectParam() ? `/register?redirect=${encodeURIComponent(redirectParam()!)}` : '/register'}
            class="text-white hover:underline ml-1"
          >
            Create account
          </a>
        </div>
      </form>
    </div>
  );
};

export default Login;
