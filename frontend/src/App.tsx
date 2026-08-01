import { useCallback, useEffect, useState } from "react";
import { api, requestError, type Requester } from "./api";
import { capabilitiesForRoles, localAdminRoles, roleLabel, rolesFromToken } from "./authz";
import AppHeader from "./components/AppHeader";
import keycloak, { initKeycloak, keycloakEnabled, roleClientId } from "./keycloak";
import DashboardView from "./views/DashboardView";
import SitesView from "./views/SitesView";
import ImportView from "./views/ImportView";
import SubnetDetailView from "./views/SubnetDetailView";
import SubnetsView from "./views/SubnetsView";
import type { ImportResult, KubernetesServiceStatus, KubernetesServiceSummary, SiteStatistics, Subnet, SubnetUsage, View } from "./types";

const viewForPath = (path: string): { view: View; subnetId?: number } => {
	const subnetMatch = path.match(/^\/subnets\/(\d+)\/?$/);
	if (subnetMatch) return { view: "subnet", subnetId: Number(subnetMatch[1]) };
	if (path.startsWith("/subnets")) return { view: "subnets" };
	if (path.startsWith("/sites")) return { view: "sites" };
	if (path.startsWith("/import")) return { view: "import" };
	return { view: "dashboard" };
};

const pathForView = (view: View, subnetId?: number) => view === "subnet" && subnetId ? `/subnets/${subnetId}` : view === "dashboard" ? "/" : `/${view}`;

const summarizeServices = (services: { status?: KubernetesServiceStatus; match_status?: KubernetesServiceStatus }[]): KubernetesServiceSummary => services.reduce<KubernetesServiceSummary>((summary, service) => {
	const status = service.status || service.match_status || "unmatched";
	summary.statuses[status] = (summary.statuses[status] || 0) + 1;
	return summary;
}, { count: services.length, statuses: {} });

export default function App() {
	const initialRoute = viewForPath(window.location.pathname);
	const [view, setView] = useState<View>(initialRoute.view);
	const [selectedSubnetId, setSelectedSubnetId] = useState<number | undefined>(initialRoute.subnetId);
	const [subnets, setSubnets] = useState<Subnet[]>([]);
	const [sites, setSites] = useState<SiteStatistics[]>([]);
	const [usage, setUsage] = useState<Record<number, SubnetUsage>>({});
	const [serviceSummaries, setServiceSummaries] = useState<Record<number, KubernetesServiceSummary>>({});
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);
	const [authReady, setAuthReady] = useState(!keycloakEnabled);
	const [authError, setAuthError] = useState<string | null>(null);
	const requester = useCallback<Requester>(async (input, init = {}) => { const client = keycloak; const token = client?.token; if (client && client.isTokenExpired(30)) { try { await client.updateToken(30); } catch { setAuthError("Your session has expired."); } } const headers = new Headers(init.headers); if (client?.token || token) headers.set("Authorization", `Bearer ${client?.token || token}`); return fetch(input, { ...init, headers }); }, []);
	const refresh = useCallback(async () => { setLoading(true); setError(null); try { const [nextSubnets, nextSites] = await Promise.all([api.subnets(requester), api.siteStatistics(requester)]); setSubnets(nextSubnets); setSites(nextSites); setUsage(Object.fromEntries(nextSubnets.map((subnet) => [subnet.id, { used: subnet.used_ips, total: subnet.total_ips }]))); setServiceSummaries({}); } catch (err) { setError(err instanceof Error ? err.message : "Unable to load inventory"); } finally { setLoading(false); } }, [requester]);
	useEffect(() => {
		if ((view !== "dashboard" && view !== "subnets") || !subnets.length) return;
		let cancelled = false;
		void Promise.all(subnets.map(async (subnet) => {
			try { return [subnet.id, { ...summarizeServices(await api.kubernetesServices(requester, subnet.id)), state: "ready" }] as const; }
			catch { return [subnet.id, { count: 0, statuses: {}, state: "unavailable" }] as const; }
		})).then((summaries) => { if (!cancelled) setServiceSummaries(Object.fromEntries(summaries)); });
		return () => { cancelled = true; };
	}, [requester, subnets, view]);
	useEffect(() => { let cancelled = false; if (!keycloakEnabled) { void refresh(); return; } initKeycloak().then((authenticated) => { if (cancelled) return; if (!authenticated) { setAuthError("Not authenticated"); return; } setAuthReady(true); void refresh(); }).catch((err) => { if (!cancelled) setAuthError(err instanceof Error ? err.message : "Unable to sign in"); }); return () => { cancelled = true; }; }, [refresh]);
	const username = keycloak?.tokenParsed?.preferred_username || keycloak?.tokenParsed?.name || "Account";
	const roles = keycloakEnabled ? rolesFromToken(keycloak?.tokenParsed, roleClientId) : localAdminRoles;
	const capabilities = capabilitiesForRoles(roles);
	useEffect(() => { const onPopState = () => { const route = viewForPath(window.location.pathname); setView(route.view); setSelectedSubnetId(route.subnetId); }; window.addEventListener("popstate", onPopState); return () => window.removeEventListener("popstate", onPopState); }, []);
	const navigate = (next: View, subnetId?: number) => { const allowed = next === "import" && !capabilities.canCreate ? "dashboard" : next; const path = pathForView(allowed, subnetId); if (window.location.pathname !== path) window.history.pushState({}, "", path); setView(allowed); setSelectedSubnetId(subnetId); };
	const saveSubnet = async (data: Partial<Subnet> & Pick<Subnet, "cidr" | "description">) => { await api.saveSubnet(requester, data); await refresh(); };
	const deleteSubnet = async (subnet: Subnet) => { const response = await api.deleteSubnet(requester, subnet.id); if (!response.ok) throw new Error(await requestError(response)); if (selectedSubnetId === subnet.id) navigate("subnets"); await refresh(); };
	const saveSite = async (data: { id?: string; name: string; description: string }) => { await api.saveSite(requester, data); await refresh(); };
	const deleteSite = async (site: SiteStatistics) => { const response = await api.deleteSite(requester, site.id); if (!response.ok) throw new Error(await requestError(response)); await refresh(); };
	const importCSV = async (file: File): Promise<ImportResult> => { const result = await api.importCSV(requester, file); await refresh(); return result; };
	const openSubnet = (subnet: Subnet) => navigate("subnet", subnet.id);
	const changePassword = () => { if (!keycloak) return; try { window.location.assign(keycloak.createAccountUrl({ redirectUri: window.location.href })); } catch { setAuthError("Keycloak account management is unavailable."); } };
	if (!authReady) return <div className="page page--center"><div className="card"><h1 className="title">Signing you in…</h1><p className="muted">{authError || "Redirecting to Keycloak."}</p></div></div>;
	const selectedSubnet = subnets.find((subnet) => subnet.id === selectedSubnetId);
		return <div className="app"><AppHeader view={view} username={username} role={roleLabel(roles)} authenticated={keycloakEnabled && Boolean(keycloak)} canImport={capabilities.canCreate} onNavigate={navigate} onChangePassword={changePassword} onLogout={() => { if (keycloak) void keycloak.logout(); }} />{authError ? <div className="content"><div className="error" role="alert">{authError}</div></div> : null}{view === "dashboard" ? <DashboardView subnets={subnets} sites={sites} usage={usage} summaries={serviceSummaries} loading={loading} error={error} canCreate={capabilities.canCreate} onSelectSubnet={openSubnet} onAddSubnet={() => navigate("subnets")} /> : view === "subnets" ? <SubnetsView subnets={subnets} sites={sites} summaries={serviceSummaries} loading={loading} error={error} canCreate={capabilities.canCreate} canEdit={capabilities.canEdit} canDelete={capabilities.canDelete} onSelect={openSubnet} onSave={saveSubnet} onDelete={deleteSubnet} /> : view === "sites" ? <SitesView sites={sites} loading={loading} error={error} canCreate={capabilities.canCreate} canEdit={capabilities.canEdit} canDelete={capabilities.canDelete} onSave={saveSite} onDelete={deleteSite} onImport={() => navigate("import")} /> : view === "import" && capabilities.canCreate ? <ImportView onImport={importCSV} /> : selectedSubnet ? <SubnetDetailView subnet={selectedSubnet} site={sites.find((site) => site.id === selectedSubnet.site_id)} requester={requester} canEdit={capabilities.canEdit} canDelete={capabilities.canDelete} onBack={() => navigate("subnets")} onRefreshUsage={refresh} /> : <main className="content"><section className="card empty-state"><h1 className="title">Subnet not found</h1><p className="muted">This subnet may have been deleted or is still loading.</p><button className="secondary" onClick={() => navigate("subnets")}>Back to subnets</button></section></main>}</div>;
}
