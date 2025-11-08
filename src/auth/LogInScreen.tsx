import { createSignal } from "solid-js";
import logoSVG from '~/assets/logo.svg';

export default function LogInScreen() {
    const [username, setUsername] = createSignal("");
    const [password, setPassword] = createSignal("");
    const [error, setError] = createSignal("");

    const handleSubmit = async (e: Event) => {
        e.preventDefault();
        setError("");

        const response = await fetch("/auth/login", {
            method: "POST",
            headers: {
                "Content-Type": "application/json",
            },
            body: JSON.stringify({ username: username(), password: password() }),
        });

        if (response.ok) {
            // Login successful, redirect or update UI
            window.location.href = "/"; // Redirect to home page
        } else {
            const data = await response.text();
            setError(data || "Login failed");
        }
    };

    return (
        <div class="flex items-center justify-center h-screen bg-neu-950">
            <form onSubmit={handleSubmit} class="bg-neu-900 p-8 rounded-lg shadow-lg w-96 border border-neu-800 space-y-4">
                <div class="space-y-2">
                    <div class="flex justify-center">
                        <img src={logoSVG} class="w-18 h-18" />
                    </div>
                    <h2 class="text-2xl font-semibold text-white text-center">Login to Unblink</h2>
                    <h3 class="text-sm text-neu-400 text-center">AI with impeccable vision to help you monitor your cameras</h3>
                </div>
                {error() && <div class="bg-red-500 text-white p-3 rounded-lg">{error()}</div>}
                <div>
                    <label for="username" class="text-sm font-medium text-neu-300">
                        Username
                    </label>
                    <input
                        type="text"
                        id="username"
                        placeholder="Username"
                        autocomplete="username"
                        class="px-3 py-1.5 mt-1 block w-full rounded-lg bg-neu-850 border border-neu-750 text-white focus:outline-none placeholder:text-neu-500"
                        value={username()}
                        onInput={(e) => setUsername(e.currentTarget.value)}
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
                        placeholder="Password"
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
                        class="w-full btn-primary"
                    >
                        Sign In
                    </button>
                </div>

                <div class="flex justify-center text-xs text-neu-500 pt-2">
                    If you don't have an account, please contact the administrator.
                </div>
            </form>
        </div>
    );
}
