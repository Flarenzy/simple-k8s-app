import { FormEvent, memo, useEffect, useMemo, useState } from "react";
import { getEnv } from "./env";
import keycloak, { initKeycloak, keycloakEnabled } from "./keycloak";

type View = "home" | "subnet" | "sites";

type Subnet = {
	id: number;
	cidr: string;
	site_id?: string;
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

type SiteStatistics = {
	id: string;
	name: string;
	description: string;
	created_at: string;
	updated_at: string;
	subnet_count: number;
	used_ip_count: number;
	total_ip_count: number;
	free_ip_count: number;
};

const API_BASE = getEnv("VITE_API_BASE", "/api/v1");
const VISIBLE_IP_WINDOW = 256;

type ParsedSubnet = {
	network: number;
	count: number;
};

function parseSubnetCidr(cidr: string): ParsedSubnet | null {
	const [addr, maskStr] = cidr.split("/");
	const mask = Number(maskStr);
	if (!addr || Number.isNaN(mask) || mask < 0 || mask > 32) {
		return null;
	}

	const octets = addr.split(".").map((n) => Number(n));
	if (octets.length !== 4 || octets.some((n) => Number.isNaN(n) || n < 0 || n > 255)) {
		return null;
	}

	const network =
		(((octets[0] << 24) | (octets[1] << 16) | (octets[2] << 8) | octets[3]) >>> 0) &
		(mask === 0 ? 0 : (0xffffffff << (32 - mask)) >>> 0);
	const count = mask === 32 ? 1 : 2 ** (32 - mask);

	return { network, count };
}

function intToIp(num: number): string {
	return `${(num >>> 24) & 255}.${(num >>> 16) & 255}.${(num >>> 8) & 255}.${num & 255}`;
}

type IpTableRowProps = {
	ipAddress: string;
	record?: IPAddress;
	isSaving: boolean;
	onSave: (ip: string, hostname: string) => void;
};

const IpTableRow = memo(
	function IpTableRow({ ipAddress, record, isSaving, onSave }: IpTableRowProps) {
		const [draft, setDraft] = useState(record?.hostname ?? "");

		useEffect(() => {
			setDraft(record?.hostname ?? "");
		}, [record?.hostname]);

		return (
			<tr>
				<td className="mono">{ipAddress}</td>
				<td>
					<input value={draft} onChange={(e) => setDraft(e.target.value)} placeholder="(unset)" />
				</td>
				<td className="muted">{record?.updated_at ? new Date(record.updated_at).toLocaleString() : ""}</td>
				<td>
					<button className="secondary" type="button" disabled={isSaving} onClick={() => onSave(ipAddress, draft)}>
						{isSaving ? "Saving..." : "Save"}
					</button>
				</td>
			</tr>
		);
	},
	(prev, next) =>
		prev.ipAddress === next.ipAddress &&
		prev.record?.id === next.record?.id &&
		prev.record?.hostname === next.record?.hostname &&
		prev.record?.updated_at === next.record?.updated_at &&
		prev.isSaving === next.isSaving,
);

export default function App() {
	const [view, setView] = useState<View>("home");
	const [subnets, setSubnets] = useState<Subnet[]>([]);
	const [loading, setLoading] = useState(false);
	const [loadError, setLoadError] = useState<string | null>(null);
	const [selectedSubnet, setSelectedSubnet] = useState<Subnet | null>(null);
	const [ips, setIps] = useState<IPAddress[]>([]);
	const [ipsLoading, setIpsLoading] = useState(false);
	const [ipsError, setIpsError] = useState<string | null>(null);
	const [savingIp, setSavingIp] = useState<string | null>(null);
	const [ipSaveError, setIpSaveError] = useState<string | null>(null);
	const [ipDeleteError, setIpDeleteError] = useState<string | null>(null);
	const [deletingSubnetId, setDeletingSubnetId] = useState<number | null>(null);
	const [deleteSubnetError, setDeleteSubnetError] = useState<string | null>(null);
	const [showCreate, setShowCreate] = useState(false);
	const [editingSubnet, setEditingSubnet] = useState<Subnet | null>(null);
	const [newCidr, setNewCidr] = useState("");
	const [newDesc, setNewDesc] = useState("");
	const [newSiteID, setNewSiteID] = useState("");
	const [saving, setSaving] = useState(false);
	const [saveError, setSaveError] = useState<string | null>(null);
	const [authReady, setAuthReady] = useState(false);
	const [authError, setAuthError] = useState<string | null>(null);
	const [windowStart, setWindowStart] = useState(0);
	const [sites, setSites] = useState<SiteStatistics[]>([]);
	const [sitesLoading, setSitesLoading] = useState(false);
	const [sitesError, setSitesError] = useState<string | null>(null);
	const [showSiteForm, setShowSiteForm] = useState(false);
	const [editingSite, setEditingSite] = useState<SiteStatistics | null>(null);
	const [siteName, setSiteName] = useState("");
	const [siteDescription, setSiteDescription] = useState("");
	const [savingSite, setSavingSite] = useState(false);
	const [siteSaveError, setSiteSaveError] = useState<string | null>(null);
	const [deletingSiteId, setDeletingSiteId] = useState<string | null>(null);

	const authClient = keycloak;

	const handleLogout = () => {
		if (!authClient) {
			return;
		}
		void authClient.logout();
	};

	const ensureToken = async () => {
		if (!keycloakEnabled || !authClient) {
			return null;
		}
		if (authClient.isTokenExpired(30)) {
			try {
				await authClient.updateToken(30);
			} catch (err) {
				setAuthError("session expired");
				return null;
			}
		}
		return authClient.token;
	};

	const fetchWithAuth = async (input: RequestInfo | URL, init: RequestInit = {}) => {
		const tk = await ensureToken();
		const headers = new Headers(init.headers || {});
		if (tk) {
			headers.set("Authorization", `Bearer ${tk}`);
		}
		return fetch(input, { ...init, headers });
	};

	const getRequestError = async (resp: Response) => {
		const text = await resp.text();
		return text || `request failed: ${resp.status}`;
	};

	const fetchSubnets = async () => {
		setLoading(true);
		setLoadError(null);
		try {
			const resp = await fetchWithAuth(`${API_BASE}/subnets`);
			if (!resp.ok) {
				setLoadError(await getRequestError(resp));
				return;
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

	const fetchSites = async () => {
		setSitesLoading(true);
		setSitesError(null);
		try {
			const resp = await fetchWithAuth(`${API_BASE}/sites/statistics`);
			if (!resp.ok) {
				setSitesError(await getRequestError(resp));
				return;
			}
			const data: SiteStatistics[] = await resp.json();
			setSites(data);
		} catch (err) {
			setSitesError(err instanceof Error ? err.message : "unknown error");
		} finally {
			setSitesLoading(false);
		}
	};

	const openSubnetForm = (subnet?: Subnet) => {
		setEditingSubnet(subnet ?? null);
		setNewCidr(subnet?.cidr ?? "");
		setNewDesc(subnet?.description ?? "");
		setNewSiteID(subnet?.site_id ?? "");
		setSaveError(null);
		setShowCreate(true);
	};

	useEffect(() => {
		let cancelled = false;
		if (!keycloakEnabled || !authClient) {
			setAuthReady(true);
			void fetchSubnets();
			void fetchSites();
			return () => {
				cancelled = true;
			};
		}

		initKeycloak()
			.then((authenticated) => {
				if (cancelled) return;
				if (!authenticated) {
					setAuthError("not authenticated");
					return;
				}
				authClient.onTokenExpired = () => {
					void authClient.updateToken(30).catch(() => {
						setAuthError("session expired");
					});
				};
				setAuthReady(true);
				void fetchSubnets();
				void fetchSites();
			})
			.catch((err) => {
				if (cancelled) return;
				setAuthError(err instanceof Error ? err.message : "auth init failed");
			});

		return () => {
			cancelled = true;
		};
		// eslint-disable-next-line react-hooks/exhaustive-deps
	}, []);

	useEffect(() => {
		if (view !== "subnet" || !selectedSubnet) return;

		const fetchIps = async () => {
			setIpsLoading(true);
			setIpsError(null);
			try {
				const resp = await fetchWithAuth(`${API_BASE}/subnets/${selectedSubnet.id}/ips`);
				if (!resp.ok) {
					setIpsError(await getRequestError(resp));
					return;
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

	useEffect(() => {
		setWindowStart(0);
	}, [selectedSubnet?.id, view]);

	const parsedSubnet = useMemo(() => {
		if (!selectedSubnet) {
			return null;
		}
		return parseSubnetCidr(selectedSubnet.cidr);
	}, [selectedSubnet]);

	const subnetIpCount = parsedSubnet?.count ?? 0;
	const maxWindowStart = subnetIpCount > VISIBLE_IP_WINDOW ? Math.floor((subnetIpCount - 1) / VISIBLE_IP_WINDOW) * VISIBLE_IP_WINDOW : 0;
	const safeWindowStart = Math.min(windowStart, maxWindowStart);
	const visibleRangeEnd = subnetIpCount === 0 ? 0 : Math.min(safeWindowStart + VISIBLE_IP_WINDOW, subnetIpCount);

	const visibleIps = useMemo(() => {
		if (!parsedSubnet) {
			return [];
		}

		const list: string[] = [];
		for (let offset = safeWindowStart; offset < visibleRangeEnd; offset += 1) {
			list.push(intToIp((parsedSubnet.network + offset) >>> 0));
		}
		return list;
	}, [parsedSubnet, safeWindowStart, visibleRangeEnd]);

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
			const resp = await fetchWithAuth(editingSubnet ? `${API_BASE}/subnets/${editingSubnet.id}` : `${API_BASE}/subnets`, {
				method: editingSubnet ? "PATCH" : "POST",
				headers: { "Content-Type": "application/json" },
				body: JSON.stringify({
					cidr: newCidr.trim(),
					description: newDesc.trim(),
					site_id: newSiteID || undefined,
				}),
			});
			if (!resp.ok) {
				setSaveError(await getRequestError(resp));
				return;
			}
				const created: Subnet = await resp.json();
				setSubnets((prev) => editingSubnet ? prev.map((subnet) => subnet.id === created.id ? created : subnet) : [created, ...prev]);
				void fetchSites();
				setShowCreate(false);
			setEditingSubnet(null);
			setNewCidr("");
			setNewDesc("");
			setNewSiteID("");
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

	const performDeleteIp = async (ipAddress: string, id: string) => {
		if (!selectedSubnet) return;
		setIpDeleteError(null);
		try {
			const resp = await fetchWithAuth(`${API_BASE}/subnets/${selectedSubnet.id}/ips/${id}`, { method: "DELETE" });
				if (!resp.ok) {
					setIpDeleteError(await getRequestError(resp));
					return;
				}
				setIps((prev) => prev.filter((ip) => ip.ip !== ipAddress));
				void fetchSites();
		} catch (err) {
			const message = err instanceof Error ? err.message : "unknown error";
			setIpDeleteError(message);
		}
	};

	const saveIpHostname = async (ip: string, hostname: string) => {
		if (!selectedSubnet) return;
		setSavingIp(ip);
		setIpSaveError(null);
		const existing = ipMap.get(ip);
		const trimmedHostname = hostname.trim();
		const usePatch = existing !== undefined && existing.hostname !== "";

		// Treat clearing hostname as a delete when the record exists.
		if (usePatch && trimmedHostname === "") {
			setSavingIp(null);
			if (existing?.id) {
				await performDeleteIp(ip, existing.id);
			}
			return;
		}

		try {
			const url = usePatch
				? `${API_BASE}/subnets/${selectedSubnet.id}/ips/${existing?.id}`
				: `${API_BASE}/subnets/${selectedSubnet.id}/ips`;
			const method = usePatch ? "PATCH" : "POST";
			const body = usePatch ? { hostname: trimmedHostname } : { ip, hostname: trimmedHostname };

			const resp = await fetchWithAuth(url, {
				method,
				headers: { "Content-Type": "application/json" },
				body: JSON.stringify(body),
			});
			if (!resp.ok) {
				setIpSaveError(await getRequestError(resp));
				return;
			}
			const saved: IPAddress = await resp.json();
				setIps((prev) => {
					const others = prev.filter((p) => p.ip !== saved.ip);
					return [saved, ...others];
				});
				void fetchSites();
		} catch (err) {
			const message = err instanceof Error ? err.message : "unknown error";
			setIpSaveError(message);
		} finally {
			setSavingIp(null);
		}
	};

	const deleteSubnet = async (subnet: Subnet) => {
		setDeletingSubnetId(subnet.id);
		setDeleteSubnetError(null);
		try {
			const resp = await fetchWithAuth(`${API_BASE}/subnets/${subnet.id}`, { method: "DELETE" });
			if (!resp.ok) {
				setDeleteSubnetError(await getRequestError(resp));
				return;
				}
				setSubnets((prev) => prev.filter((s) => s.id !== subnet.id));
				void fetchSites();
			if (selectedSubnet?.id === subnet.id) {
				setSelectedSubnet(null);
				setView("home");
				setIps([]);
			}
		} catch (err) {
			const message = err instanceof Error ? err.message : "unknown error";
			setDeleteSubnetError(message);
		} finally {
			setDeletingSubnetId(null);
		}
	};

	const openSiteForm = (site?: SiteStatistics) => {
		setEditingSite(site ?? null);
		setSiteName(site?.name ?? "");
		setSiteDescription(site?.description ?? "");
		setSiteSaveError(null);
		setShowSiteForm(true);
	};

	const saveSite = async (event: FormEvent<HTMLFormElement>) => {
		event.preventDefault();
		const name = siteName.trim();
		if (!name) {
			setSiteSaveError("Name is required");
			return;
		}
		setSavingSite(true);
		setSiteSaveError(null);
		try {
			const resp = await fetchWithAuth(
				editingSite ? `${API_BASE}/sites/${editingSite.id}` : `${API_BASE}/sites`,
				{
					method: editingSite ? "PATCH" : "POST",
					headers: { "Content-Type": "application/json" },
					body: JSON.stringify({ name, description: siteDescription.trim() }),
				},
			);
			if (!resp.ok) {
				setSiteSaveError(await getRequestError(resp));
				return;
			}
			setShowSiteForm(false);
			await fetchSites();
		} catch (err) {
			setSiteSaveError(err instanceof Error ? err.message : "unknown error");
		} finally {
			setSavingSite(false);
		}
	};

	const deleteSite = async (site: SiteStatistics) => {
		setDeletingSiteId(site.id);
		setSitesError(null);
		try {
			const resp = await fetchWithAuth(`${API_BASE}/sites/${site.id}`, { method: "DELETE" });
			if (!resp.ok) {
				setSitesError(await getRequestError(resp));
				return;
			}
			setSites((prev) => prev.filter((item) => item.id !== site.id));
		} catch (err) {
			setSitesError(err instanceof Error ? err.message : "unknown error");
		} finally {
			setDeletingSiteId(null);
		}
	};

	if (!authReady) {
		return (
			<div className="page page--center">
				<div className="card">
					<h1 className="title">Signing you in…</h1>
					{authError ? <div className="error">{authError}</div> : <p className="muted">Redirecting to Keycloak.</p>}
				</div>
			</div>
		);
	}

	if (view === "home") {
		return (
			<div className="page">
				<header className="header">
					<div>
						<div className="eyebrow">{keycloakEnabled ? "Logged in" : "Ready"}</div>
						<div className="title">IPAM</div>
					</div>
					{keycloakEnabled && authClient ? (
						<button className="secondary" onClick={handleLogout}>
							Log out
						</button>
					) : null}
				</header>
				<main className="panel">
					<div className="panel__title-row">
						<div style={{ display: "flex", alignItems: "center", gap: "8px" }}>
							<h1 className="panel__title">Subnets</h1>
							<div className="pill">{subnets.length}</div>
						</div>
						<div className="button-group">
							<button className="secondary" onClick={() => setView("sites")}>Sites</button>
							<button className="primary" onClick={() => openSubnetForm()}>+ Add subnet</button>
						</div>
					</div>
					<p className="muted">Current subnets and usage (ID hidden from UI).</p>

					{loading ? <div className="muted">Loading...</div> : null}
					{loadError ? <div className="error">Failed to load: {loadError}</div> : null}
					{deleteSubnetError ? <div className="error">Delete failed: {deleteSubnetError}</div> : null}

					{!loading && !loadError ? (
						<table className="table">
							<thead>
								<tr>
									<th>CIDR</th>
									<th>Description</th>
									<th>Site</th>
									<th>Usage</th>
									<th>Updated</th>
									<th />
								</tr>
							</thead>
							<tbody>
								{subnets.length === 0 ? (
									<tr>
										<td colSpan={6} className="muted">
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
											<td>{sites.find((site) => site.id === subnet.site_id)?.name || "Unassigned"}</td>
											<td className="muted">N/A</td>
											<td className="muted">{new Date(subnet.updated_at).toLocaleString()}</td>
											<td>
												<button className="secondary" type="button" onClick={(e) => { e.stopPropagation(); openSubnetForm(subnet); }}>Edit</button>
												<button
													className="secondary danger"
													type="button"
													onClick={(e) => {
														e.stopPropagation();
														void deleteSubnet(subnet);
													}}
													disabled={deletingSubnetId === subnet.id}
												>
													{deletingSubnetId === subnet.id ? "Deleting..." : "Delete"}
												</button>
											</td>
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
														<p className="eyebrow">{editingSubnet ? "Edit subnet" : "Create subnet"}</p>
														<h2 className="title">{editingSubnet ? editingSubnet.cidr : "New subnet"}</h2>
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
								<span>Site</span>
								<select value={newSiteID} onChange={(e) => setNewSiteID(e.target.value)}>
									<option value="">Unassigned</option>
									{sites.map((site) => <option key={site.id} value={site.id}>{site.name}</option>)}
								</select>
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

	if (view === "sites") {
		return (
			<div className="page">
				<header className="header">
					<button className="secondary" onClick={() => setView("home")}>← Subnets</button>
					{keycloakEnabled && authClient ? <button className="secondary" onClick={handleLogout}>Log out</button> : null}
				</header>
				<main className="panel">
					<div className="panel__title-row">
						<div>
							<h1 className="panel__title">Sites</h1>
							<p className="muted">Network inventory grouped by location.</p>
						</div>
						<button className="primary" onClick={() => openSiteForm()}>+ Add site</button>
					</div>
					{sitesLoading ? <div className="muted">Loading sites...</div> : null}
					{sitesError ? <div className="error">Failed to load sites: {sitesError}</div> : null}
					{!sitesLoading && !sitesError ? (
						<table className="table">
							<thead><tr><th>Name</th><th>Subnets</th><th>IP usage</th><th>Available</th><th>Updated</th><th /></tr></thead>
							<tbody>
								{sites.length === 0 ? <tr><td colSpan={6} className="muted">No sites yet.</td></tr> : sites.map((site) => (
									<tr key={site.id}>
										<td><div>{site.name}</div><div className="muted">{site.description || "No description"}</div></td>
										<td>{site.subnet_count}</td>
										<td>{site.used_ip_count} / {site.total_ip_count}</td>
										<td>{site.free_ip_count}</td>
										<td className="muted">{new Date(site.updated_at).toLocaleString()}</td>
										<td className="actions"><button className="secondary" onClick={() => openSiteForm(site)}>Edit</button><button className="secondary danger" onClick={() => void deleteSite(site)} disabled={deletingSiteId === site.id}>{deletingSiteId === site.id ? "Deleting..." : "Delete"}</button></td>
									</tr>
								))}
							</tbody>
						</table>
					) : null}
				</main>
				{showSiteForm ? <div className="modal"><div className="modal__backdrop" onClick={() => setShowSiteForm(false)} /><form className="modal__content" onSubmit={saveSite}>
					<div className="card__header"><p className="eyebrow">{editingSite ? "Edit site" : "Create site"}</p><h2 className="title">{editingSite ? editingSite.name : "New site"}</h2></div>
					<label className="field"><span>Name</span><input value={siteName} onChange={(e) => setSiteName(e.target.value)} placeholder="Belgrade office" required /></label>
					<label className="field"><span>Description</span><input value={siteDescription} onChange={(e) => setSiteDescription(e.target.value)} placeholder="Primary office network" /></label>
					{siteSaveError ? <div className="error">Failed: {siteSaveError}</div> : null}
					<div className="modal__actions"><button type="button" className="secondary" onClick={() => setShowSiteForm(false)} disabled={savingSite}>Cancel</button><button type="submit" className="primary" disabled={savingSite}>{savingSite ? "Saving..." : "Save"}</button></div>
				</form></div> : null}
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
					{keycloakEnabled && authClient ? (
						<button className="secondary" onClick={handleLogout}>
							Log out
						</button>
					) : null}
				</header>
				<main className="panel">
					<div className="panel__title-row">
						<div style={{ display: "flex", alignItems: "center", gap: "8px" }}>
							<h1 className="panel__title">{selectedSubnet.cidr}</h1>
							<div className="pill">{subnetIpCount || ips.length}</div>
						</div>
						<div className="muted">Subnet: {selectedSubnet.description || "No description"}</div>
					</div>

					{ipsLoading ? <div className="muted">Loading IPs...</div> : null}
					{ipsError ? <div className="error">Failed to load IPs: {ipsError}</div> : null}
					{ipSaveError ? <div className="error">Save failed: {ipSaveError}</div> : null}
					{ipDeleteError ? <div className="error">Delete failed: {ipDeleteError}</div> : null}
					{!parsedSubnet ? <div className="error">Cannot render IPs for this subnet.</div> : null}

					{parsedSubnet ? (
						<>
							{subnetIpCount > VISIBLE_IP_WINDOW ? (
								<div className="panel__title-row">
									<div className="muted">
										Showing {safeWindowStart + 1}-{visibleRangeEnd} of {subnetIpCount}
									</div>
									<div style={{ display: "flex", gap: "8px" }}>
										<button
											className="secondary"
											type="button"
											onClick={() => setWindowStart((prev) => Math.max(prev - VISIBLE_IP_WINDOW, 0))}
											disabled={safeWindowStart === 0}
										>
											Previous
										</button>
										<button
											className="secondary"
											type="button"
											onClick={() =>
												setWindowStart((prev) => Math.min(prev + VISIBLE_IP_WINDOW, maxWindowStart))
											}
											disabled={safeWindowStart >= maxWindowStart}
										>
											Next
										</button>
									</div>
								</div>
							) : null}

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
									{visibleIps.length === 0 ? (
										<tr>
											<td colSpan={4} className="muted">
												No IPs yet.
											</td>
										</tr>
									) : (
										visibleIps.map((ipAddress) => (
											<IpTableRow
												key={ipAddress}
												ipAddress={ipAddress}
												record={ipMap.get(ipAddress)}
												isSaving={savingIp === ipAddress}
												onSave={(ip, hostname) => {
													void saveIpHostname(ip, hostname);
												}}
											/>
										))
									)}
								</tbody>
							</table>
						</>
					) : null}
				</main>
			</div>
		);
	}

	// Should never reach here because auth flow returns earlier.
	return null;
}
