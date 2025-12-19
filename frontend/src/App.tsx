import { FormEvent, useEffect, useMemo, useState } from "react";

type View = "login" | "home";

type Subnet = {
	id: number;
	cidr: string;
	description: string;
	created_at: string;
	updated_at: string;
};

const ADMIN_USERNAME = "admin";
const API_BASE = "/api/v1";

export default function App() {
	const [username, setUsername] = useState("");
	const [password, setPassword] = useState("");
	const [view, setView] = useState<View>("login");
	const [error, setError] = useState<string | null>(null);
	const [subnets, setSubnets] = useState<Subnet[]>([]);
	const [loading, setLoading] = useState(false);
	const [loadError, setLoadError] = useState<string | null>(null);

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
		setSubnets([]);
		setLoadError(null);
	};

	useEffect(() => {
		if (view !== "home") return;

		const fetchSubnets = async () => {
			setLoading(true);
			setLoadError(null);
			try {
				const resp = await fetch(`${API_BASE}/subnets`);
				if (!resp.ok) {
					throw new Error(`request failed: ${resp.status}`);
				}
				const data: Subnet[] = await resp.json();
				setSubnets(data);
			} catch (err) {
				const message = err instanceof Error ? err.message : "unknown error";
				setLoadError(message);
			} finally {
				setLoading(false);
			}
		};

		fetchSubnets();
	}, [view]);

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
					<div className="panel__title-row">
						<h1 className="panel__title">Subnets</h1>
						<div className="pill">{subnets.length}</div>
					</div>
					<p className="muted">Current subnets and usage (ID hidden from UI).</p>

					{loading ? <div className="muted">Loading...</div> : null}
					{loadError ? <div className="error">Failed to load: {loadError}</div> : null}

					{!loading && !loadError ? (
						<table className="table">
							<thead>
								<tr>
									<th>CIDR</th>
									<th>Description</th>
									<th>Usage</th>
									<th>Updated</th>
								</tr>
							</thead>
							<tbody>
								{subnets.length === 0 ? (
									<tr>
										<td colSpan={4} className="muted">
											No subnets yet.
										</td>
									</tr>
								) : (
									subnets.map((subnet) => (
										<tr key={subnet.id}>
											<td className="mono">{subnet.cidr}</td>
											<td>{subnet.description || "—"}</td>
											<td className="muted">N/A</td>
											<td className="muted">{new Date(subnet.updated_at).toLocaleString()}</td>
										</tr>
									))
								)}
							</tbody>
						</table>
					) : null}
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
