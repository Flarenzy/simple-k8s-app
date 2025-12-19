import { FormEvent, useMemo, useState } from "react";

type View = "login" | "home";

const ADMIN_USERNAME = "admin";

export default function App() {
	const [username, setUsername] = useState("");
	const [password, setPassword] = useState("");
	const [view, setView] = useState<View>("login");
	const [error, setError] = useState<string | null>(null);

	const isAdmin = useMemo(() => view === "home", [view]);

	const handleSubmit = (e: FormEvent<HTMLFormElement>) => {
		e.preventDefault();
		setError(null);

		if (username.trim() === ADMIN_USERNAME) {
			setView("home");
			return;
		}

		setError("Invalid username. Use admin to continue.");
	};

	const reset = () => {
		setUsername("");
		setPassword("");
		setView("login");
		setError(null);
	};

	if (view === "home") {
		return (
			<div className="page">
				<header className="header">
					<div>
						<div className="eyebrow">Logged in as</div>
						<div className="title">{ADMIN_USERNAME}</div>
					</div>
					<button className="secondary" onClick={reset}>
						Log out
					</button>
				</header>
				<main className="panel">
					<h1 className="panel__title">Welcome, admin</h1>
					<p className="muted">Frontend wiring goes here.</p>
				</main>
			</div>
		);
	}

	return (
		<div className="page page--center">
			<form className="card" onSubmit={handleSubmit}>
				<div className="card__header">
					<p className="eyebrow">Simple IPAM</p>
					<h1 className="title">Sign in</h1>
					<p className="muted">Use username admin. Password is ignored.</p>
				</div>

				<label className="field">
					<span>Username</span>
					<input
						value={username}
						onChange={(e) => setUsername(e.target.value)}
						placeholder="admin"
						autoFocus
						required
					/>
				</label>

				<label className="field">
					<span>Password</span>
					<input
						type="password"
						value={password}
						onChange={(e) => setPassword(e.target.value)}
						placeholder="••••••••"
					/>
				</label>

				{error ? <div className="error">{error}</div> : null}

				<button className="primary" type="submit">
					Continue
				</button>
			</form>
		</div>
	);
}
