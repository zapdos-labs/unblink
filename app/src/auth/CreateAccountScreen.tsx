import { createSignal } from "solid-js";
import { createAccount } from "../signals/authSignals";

interface CreateAccountScreenProps {
	onSuccess?: () => void;
	onSwitchToLogin?: () => void;
}

export default function CreateAccountScreen(props: CreateAccountScreenProps) {
	const [name, setName] = createSignal("");
	const [email, setEmail] = createSignal("");
	const [password, setPassword] = createSignal("");
	const [confirmPassword, setConfirmPassword] = createSignal("");
	const [error, setError] = createSignal("");
	const [isLoading, setIsLoading] = createSignal(false);

	const handleSubmit = async (e: Event) => {
		e.preventDefault();
		setError("");

		// Validate passwords match
		if (password() !== confirmPassword()) {
			setError("Passwords do not match");
			return;
		}

		// Validate password length
		if (password().length < 8) {
			setError("Password must be at least 8 characters");
			return;
		}

		setIsLoading(true);

		try {
			const success = await createAccount(email(), password(), name());
			if (success) {
				props.onSuccess?.();
			} else {
				setError("Registration failed");
			}
		} catch (err) {
			setError(err instanceof Error ? err.message : "Registration failed");
		} finally {
			setIsLoading(false);
		}
	};

	return (
		<div class="flex items-center justify-center h-screen bg-neu-950">
			<form
				onSubmit={handleSubmit}
				class="bg-neu-900 p-8 rounded-lg shadow-lg w-96 border border-neu-800 space-y-4"
			>
				<div class="space-y-2">
					<div class="flex justify-center">
						<img src="/logo.svg" class="w-18 h-18" alt="Logo" />
					</div>
					<h2 class="text-2xl font-semibold text-white text-center">Create Account</h2>
					<h3 class="text-sm text-neu-400 text-center">
						Sign up to get started with your account
					</h3>
				</div>
				{error() && <div class="bg-red-500 text-white p-3 rounded-lg">{error()}</div>}
				<div>
					<label for="name" class="text-sm font-medium text-neu-300">
						Name
					</label>
					<input
						type="text"
						id="name"
						placeholder="Your name"
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
						autocomplete="email"
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
						placeholder="Password"
						autocomplete="new-password"
						class="px-3 py-1.5 mt-1 block w-full rounded-lg bg-neu-850 border border-neu-750 text-white focus:outline-none placeholder:text-neu-500"
						value={password()}
						onInput={(e) => setPassword(e.currentTarget.value)}
						required
						minLength={8}
					/>
				</div>
				<div>
					<label for="confirm-password" class="text-sm font-medium text-neu-300">
						Confirm Password
					</label>
					<input
						type="password"
						id="confirm-password"
						placeholder="Password"
						autocomplete="new-password"
						class="px-3 py-1.5 mt-1 block w-full rounded-lg bg-neu-850 border border-neu-750 text-white focus:outline-none placeholder:text-neu-500"
						value={confirmPassword()}
						onInput={(e) => setConfirmPassword(e.currentTarget.value)}
						required
					/>
				</div>
				<div class="pt-4">
					<button
						type="submit"
						disabled={isLoading()}
						class="w-full px-4 py-2 rounded-xl border border-neu-750 bg-neu-800 hover:bg-neu-850 flex items-center justify-center space-x-2 disabled:opacity-50 disabled:cursor-not-allowed"
					>
						<span class="text-white">{isLoading() ? "Creating account..." : "Create Account"}</span>
					</button>
				</div>

				{props.onSwitchToLogin && (
					<div class="flex justify-center text-xs text-neu-500 pt-2">
						Already have an account?{" "}
						<button
							type="button"
							onClick={props.onSwitchToLogin}
							class="ml-1 text-neu-400 hover:text-white underline"
						>
							Sign In
						</button>
					</div>
				)}
			</form>
		</div>
	);
}
