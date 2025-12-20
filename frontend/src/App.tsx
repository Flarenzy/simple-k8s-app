import { FormEvent, useEffect, useMemo, useState } from "react";

type View = "login" | "home" | "subnet";

type Subnet = {
	id: number;
	cidr: string;
	description: string;
	created_at: string;
	updated_at: string;
};

type IPAddress = {
	id: string;
	ip: string;
	hostname: string;
	subnet_id: number;
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
	const [selectedSubnet, setSelectedSubnet] = useState<Subnet | null>(null);
	const [ips, setIps] = useState<IPAddress[]>([]);
	const [ipsLoading, setIpsLoading] = useState(false);
	const [ipsError, setIpsError] = useState<string | null>(null);
	const [hostnameDrafts, setHostnameDrafts] = useState<Record<string, string>>({});
	const [savingIp, setSavingIp] = useState<string | null>(null);
	const [ipSaveError, setIpSaveError] = useState<string | null>(null);
	const [showCreate, setShowCreate] = useState(false);
	const [newCidr, setNewCidr] = useState("");
	const [newDesc, setNewDesc] = useState("");
	const [saving, setSaving] = useState(false);
	const [saveError, setSaveError] = useState<string | null>(null);

	const isAdmin = useMemo(() => view === "home", [view]);

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
		setSelectedSubnet(null);
		setIps([]);
		setIpsError(null);
		setIpsLoading(false);
		setHostnameDrafts({});
		setSavingIp(null);
		setIpSaveError(null);
		setShowCreate(false);
		setNewCidr("");
		setNewDesc("");
		setSaving(false);
		setSaveError(null);
	};

	useEffect(() => {
		if (view !== "home") return;

		fetchSubnets();
	}, [view]);

	useEffect(() => {
		if (view !== "subnet" || !selectedSubnet) return;

		const fetchIps = async () => {
			setIpsLoading(true);
			setIpsError(null);
			try {
				const resp = await fetch(`${API_BASE}/subnets/${selectedSubnet.id}/ips`);
				if (!resp.ok) {
					throw new Error(`request failed: ${resp.status}`);
				}
				const data: IPAddress[] = await resp.json();
				setIps(data);
			} catch (err) {
				const message = err instanceof Error ? err.message : "unknown error";
				setIpsError(message);
			} finally {
				setIpsLoading(false);
			}
		};

		fetchIps();
	}, [selectedSubnet, view]);

	const allIps = useMemo(() => {
		if (!selectedSubnet) return [];
		const [addr, maskStr] = selectedSubnet.cidr.split("/");
		const mask = Number(maskStr);
		if (!addr || Number.isNaN(mask) || mask < 0 || mask > 32) {
			return [];
		}
		const octets = addr.split(".").map((n) => Number(n));
		if (octets.length !== 4 || octets.some((n) => Number.isNaN(n) || n < 0 || n > 255)) {
			return [];
		}
		const toInt = (o: number[]) => ((o[0] << 24) | (o[1] << 16) | (o[2] << 8) | o[3]) >>> 0;
		const toIP = (num: number) =>
			`${(num >>> 24) & 255}.${(num >>> 16) & 255}.${(num >>> 8) & 255}.${num & 255}`;
		const network = toInt(octets) & (mask === 0 ? 0 : (0xffffffff << (32 - mask)) >>> 0);
		const count = mask === 32 ? 1 : 2 ** (32 - mask);
		// Guard against extremely large subnets to avoid locking the UI.
		if (count > 65536) {
			return [];
		}
		const list: string[] = [];
		for (let i = 0; i < count; i += 1) {
			list.push(toIP((network + i) >>> 0));
		}
		return list;
	}, [selectedSubnet]);

	const ipMap = useMemo(() => {
		const map = new Map<string, IPAddress>();
		for (const ip of ips) {
			map.set(ip.ip, ip);
		}
		return map;
	}, [ips]);

	const createSubnet = async () => {
		setSaving(true);
		setSaveError(null);
		try {
			const resp = await fetch(`${API_BASE}/subnets`, {
				method: "POST",
				headers: { "Content-Type": "application/json" },
				body: JSON.stringify({ cidr: newCidr.trim(), description: newDesc.trim() }),
			});
			if (!resp.ok) {
				const text = await resp.text();
				throw new Error(text || `request failed: ${resp.status}`);
			}
			const created: Subnet = await resp.json();
			setSubnets((prev) => [created, ...prev]);
			setShowCreate(false);
			setNewCidr("");
			setNewDesc("");
		} catch (err) {
			const message = err instanceof Error ? err.message : "unknown error";
			setSaveError(message);
		} finally {
			setSaving(false);
		}
	};

	const handleCreateSubmit = (e: FormEvent<HTMLFormElement>) => {
		e.preventDefault();
		if (!newCidr.trim()) {
			setSaveError("CIDR is required");
			return;
		}
		void createSubnet();
	};

	const saveIpHostname = async (ip: string, hostname: string) => {
		if (!selectedSubnet) return;
		setSavingIp(ip);
		setIpSaveError(null);
		const existing = ipMap.get(ip);
		const usePatch = existing !== undefined && existing.hostname !== "";
		try {
			const url = usePatch
				? `${API_BASE}/subnets/${selectedSubnet.id}/ips/${existing?.id}`
				: `${API_BASE}/subnets/${selectedSubnet.id}/ips`;
			const method = usePatch ? "PATCH" : "POST";
			const body = usePatch ? { hostname } : { ip, hostname };

			const resp = await fetch(url, {
				method,
				headers: { "Content-Type": "application/json" },
				body: JSON.stringify(body),
			});
			if (!resp.ok) {
				const text = await resp.text();
				throw new Error(text || `request failed: ${resp.status}`);
			}
			const saved: IPAddress = await resp.json();
			setIps((prev) => {
				const others = prev.filter((p) => p.ip !== saved.ip);
				return [saved, ...others];
			});
		} catch (err) {
			const message = err instanceof Error ? err.message : "unknown error";
			setIpSaveError(message);
		} finally {
			setSavingIp(null);
		}
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
					<div className="panel__title-row">
						<div style={{ display: "flex", alignItems: "center", gap: "8px" }}>
							<h1 className="panel__title">Subnets</h1>
							<div className="pill">{subnets.length}</div>
						</div>
						<button className="primary" onClick={() => setShowCreate(true)}>
							+ Add
						</button>
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
										<tr
											key={subnet.id}
											className="row-click"
											onClick={() => {
												setSelectedSubnet(subnet);
												setView("subnet");
											}}
										>
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

				{showCreate ? (
					<div className="modal">
						<div className="modal__backdrop" onClick={() => setShowCreate(false)} />
						<form className="modal__content" onSubmit={handleCreateSubmit}>
							<div className="card__header">
								<p className="eyebrow">Create subnet</p>
								<h2 className="title">New subnet</h2>
								<p className="muted">Backend validates the CIDR.</p>
							</div>

							<label className="field">
								<span>CIDR</span>
								<input
									value={newCidr}
									onChange={(e) => setNewCidr(e.target.value)}
									placeholder="10.0.0.0/24"
									required
								/>
							</label>

							<label className="field">
								<span>Description</span>
								<input
									value={newDesc}
									onChange={(e) => setNewDesc(e.target.value)}
									placeholder="Office network"
								/>
							</label>

							{saveError ? <div className="error">Failed: {saveError}</div> : null}

							<div className="modal__actions">
								<button type="button" className="secondary" onClick={() => setShowCreate(false)} disabled={saving}>
									Cancel
								</button>
								<button type="submit" className="primary" disabled={saving}>
									{saving ? "Saving..." : "Save"}
								</button>
							</div>
						</form>
					</div>
				) : null}
			</div>
		);
	}

	if (view === "subnet" && selectedSubnet) {
		return (
			<div className="page">
				<header className="header">
					<div>
						<button className="secondary" onClick={() => setView("home")}>
							← Back
						</button>
					</div>
					<button className="secondary" onClick={reset}>
						Log out
					</button>
				</header>
				<main className="panel">
					<div className="panel__title-row">
						<div style={{ display: "flex", alignItems: "center", gap: "8px" }}>
							<h1 className="panel__title">{selectedSubnet.cidr}</h1>
							<div className="pill">{allIps.length || ips.length}</div>
						</div>
						<div className="muted">Subnet: {selectedSubnet.description || "No description"}</div>
					</div>

					{ipsLoading ? <div className="muted">Loading IPs...</div> : null}
					{ipsError ? <div className="error">Failed to load IPs: {ipsError}</div> : null}
					{ipSaveError ? <div className="error">Save failed: {ipSaveError}</div> : null}
					{allIps.length === 0 ? <div className="error">Cannot render IPs for this subnet.</div> : null}

					<table className="table">
						<thead>
							<tr>
								<th>IP</th>
								<th>Hostname</th>
								<th>Updated</th>
								<th />
							</tr>
						</thead>
						<tbody>
							{allIps.length === 0 ? (
								<tr>
									<td colSpan={4} className="muted">
										No IPs yet.
									</td>
								</tr>
							) : (
								allIps.map((ipAddress) => {
									const record = ipMap.get(ipAddress);
									const draft = hostnameDrafts[ipAddress] ?? record?.hostname ?? "";
									return (
										<tr key={ipAddress}>
											<td className="mono">{ipAddress}</td>
											<td>
												<input
													value={draft}
													onChange={(e) =>
														setHostnameDrafts((prev) => ({ ...prev, [ipAddress]: e.target.value }))
													}
													placeholder="(unset)"
												/>
											</td>
											<td className="muted">
												{record?.updated_at ? new Date(record.updated_at).toLocaleString() : ""}
											</td>
											<td>
												<button
													className="secondary"
													type="button"
													disabled={savingIp === ipAddress}
													onClick={() => saveIpHostname(ipAddress, draft)}
												>
													{savingIp === ipAddress ? "Saving..." : "Save"}
												</button>
											</td>
										</tr>
									);
								})
							)}
						</tbody>
					</table>
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
