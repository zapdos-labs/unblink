import { createSignal, onMount, type Component } from 'solid-js';
import { register, authState, initAuth } from '../shared';

const Register: Component = () => {
  const [email, setEmail] = createSignal('');
  const [password, setPassword] = createSignal('');
  const [name, setName] = createSignal('');
  const [error, setError] = createSignal('');
  const [loading, setLoading] = createSignal(false);
  const [redirect, setRedirect] = createSignal<string | null>(null);

  onMount(async () => {
    await initAuth();

    // If already authenticated, redirect to main page
    if (authState().isAuthenticated) {
      window.location.href = redirect() || '/';
      return;
    }

    const params = new URLSearchParams(window.location.search);
    setRedirect(params.get('redirect'));
  });

  const loginHref = () => {
    const r = redirect();
    return r ? `/login?redirect=${encodeURIComponent(r)}` : '/login';
  };

  const handleSubmit = async (e: Event) => {
    e.preventDefault();
    setError('');
    setLoading(true);

    const result = await register(email(), password(), name());

    if (result.success) {
      const url = new URL('/login', window.location.origin);
      url.searchParams.set('message', 'Registration successful! Please log in.');
      const redirectUrl = redirect();
      if (redirectUrl) {
        url.searchParams.set('redirect', redirectUrl);
      }
      window.location.href = url.toString();
    } else {
      setError(result.message || 'Registration failed');
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
          <h2 class="text-2xl font-semibold text-white text-center">Create Account</h2>
          <h3 class="text-sm text-neu-400 text-center">Join Unblink to monitor your cameras</h3>
        </div>

        {error() && (
          <div class="bg-red-900/30 text-red-400 border border-red-800 p-3 rounded-lg text-sm">
            {error()}
          </div>
        )}

        <div>
          <label for="name" class="text-sm font-medium text-neu-300">
            Name
          </label>
          <input
            type="text"
            id="name"
            placeholder="Your full name"
            autocomplete="name"
            class="px-3 py-1.5 mt-1 block w-full rounded-lg bg-neu-850 border border-neu-750 text-white focus:outline-none placeholder:text-neu-500"
            value={name()}
            onInput={(e) => setName(e.currentTarget.value)}
            required
          />
        </div>

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
            placeholder="Create a password"
            autocomplete="new-password"
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
            {loading() ? 'Creating account...' : 'Create Account'}
          </button>
        </div>

        <div class="flex justify-center text-xs text-neu-500 pt-2">
          Already have an account?{' '}
          <a
            href={loginHref()}
            class="text-white hover:underline ml-1"
          >
            Sign in
          </a>
        </div>
      </form>
    </div>
  );
};

export default Register;
