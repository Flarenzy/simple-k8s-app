import { useCallback, useEffect, useState } from "react";
import { api, requestError, type Requester } from "./api";
import AppHeader from "./components/AppHeader";
import keycloak, { initKeycloak, keycloakEnabled } from "./keycloak";
import DashboardView from "./views/DashboardView";
import SitesView from "./views/SitesView";
import ImportView from "./views/ImportView";
import SubnetDetailView from "./views/SubnetDetailView";
import SubnetsView from "./views/SubnetsView";
import type { ImportResult, SiteStatistics, Subnet, SubnetUsage, View } from "./types";

export default function App() {
	const [view, setView] = useState<View>("dashboard");
	const [subnets, setSubnets] = useState<Subnet[]>([]);
	const [sites, setSites] = useState<SiteStatistics[]>([]);
	const [usage, setUsage] = useState<Record<number, SubnetUsage>>({});
	const [selectedSubnet, setSelectedSubnet] = useState<Subnet | null>(null);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);
	const [authReady, setAuthReady] = useState(!keycloakEnabled);
	const [authError, setAuthError] = useState<string | null>(null);
	const requester = useCallback<Requester>(async (input, init = {}) => { const client = keycloak; const token = client?.token; if (client && client.isTokenExpired(30)) { try { await client.updateToken(30); } catch { setAuthError("Your session has expired."); } } const headers = new Headers(init.headers); if (client?.token || token) headers.set("Authorization", `Bearer ${client?.token || token}`); return fetch(input, { ...init, headers }); }, []);
	const refresh = useCallback(async () => { setLoading(true); setError(null); try { const [nextSubnets, nextSites] = await Promise.all([api.subnets(requester), api.siteStatistics(requester)]); setSubnets(nextSubnets); setSites(nextSites); setUsage(Object.fromEntries(nextSubnets.map((subnet) => [subnet.id, { used: subnet.used_ips, total: subnet.total_ips }]))); } catch (err) { setError(err instanceof Error ? err.message : "Unable to load inventory"); } finally { setLoading(false); } }, [requester]);
	useEffect(() => { let cancelled = false; if (!keycloakEnabled) { void refresh(); return; } initKeycloak().then((authenticated) => { if (cancelled) return; if (!authenticated) { setAuthError("Not authenticated"); return; } setAuthReady(true); void refresh(); }).catch((err) => { if (!cancelled) setAuthError(err instanceof Error ? err.message : "Unable to sign in"); }); return () => { cancelled = true; }; }, [refresh]);
	const username = keycloak?.tokenParsed?.preferred_username || keycloak?.tokenParsed?.name || "Account";
	const navigate = (next: View) => { setView(next); if (next !== "subnet") setSelectedSubnet(null); };
	const saveSubnet = async (data: Partial<Subnet> & Pick<Subnet, "cidr" | "description">) => { await api.saveSubnet(requester, data); await refresh(); };
	const deleteSubnet = async (subnet: Subnet) => { const response = await api.deleteSubnet(requester, subnet.id); if (!response.ok) throw new Error(await requestError(response)); if (selectedSubnet?.id === subnet.id) setSelectedSubnet(null); await refresh(); };
	const saveSite = async (data: { id?: string; name: string; description: string }) => { await api.saveSite(requester, data); await refresh(); };
	const deleteSite = async (site: SiteStatistics) => { const response = await api.deleteSite(requester, site.id); if (!response.ok) throw new Error(await requestError(response)); await refresh(); };
	const importCSV = async (file: File): Promise<ImportResult> => { const result = await api.importCSV(requester, file); await refresh(); return result; };
	const openSubnet = (subnet: Subnet) => { setSelectedSubnet(subnet); setView("subnet"); };
	const changePassword = () => { if (!keycloak) return; try { window.location.assign(keycloak.createAccountUrl({ redirectUri: window.location.href })); } catch { setAuthError("Keycloak account management is unavailable."); } };
	if (!authReady) return <div className="page page--center"><div className="card"><h1 className="title">Signing you in…</h1><p className="muted">{authError || "Redirecting to Keycloak."}</p></div></div>;
		return <div className="app"><AppHeader view={view} username={username} authenticated={keycloakEnabled && Boolean(keycloak)} onNavigate={navigate} onChangePassword={changePassword} onLogout={() => { if (keycloak) void keycloak.logout(); }} />{authError ? <div className="content"><div className="error">{authError}</div></div> : null}{view === "dashboard" ? <DashboardView subnets={subnets} sites={sites} usage={usage} loading={loading} error={error} onSelectSubnet={openSubnet} onAddSubnet={() => setView("subnets")} /> : view === "subnets" ? <SubnetsView subnets={subnets} sites={sites} loading={loading} error={error} onSelect={openSubnet} onSave={saveSubnet} onDelete={deleteSubnet} /> : view === "sites" ? <SitesView sites={sites} loading={loading} error={error} onSave={saveSite} onDelete={deleteSite} onImport={() => setView("import")} /> : view === "import" ? <ImportView onImport={importCSV} /> : selectedSubnet ? <SubnetDetailView subnet={selectedSubnet} site={sites.find((site) => site.id === selectedSubnet.site_id)} requester={requester} onBack={() => navigate("subnets")} onRefreshUsage={refresh} /> : null}</div>;
}
