import { getEnv } from "./env";
import type { IPAddress, ImportResult, KubernetesServiceObservation, Site, SiteStatistics, Subnet } from "./types";

const API_BASE = getEnv("VITE_API_BASE", "/api/v1");

export type Requester = (input: RequestInfo | URL, init?: RequestInit) => Promise<Response>;

export async function requestError(response: Response) {
	const text = await response.text();
	if (!text) return `request failed: ${response.status}`;
	try {
		const parsed = JSON.parse(text) as { error?: string };
		return parsed.error || text;
	} catch {
		return text;
	}
}

async function json<T>(requester: Requester, path: string, init?: RequestInit): Promise<T> {
	const response = await requester(`${API_BASE}${path}`, init);
	if (!response.ok) throw new Error(await requestError(response));
	return response.json() as Promise<T>;
}

function mapIPAddress(record: IPAddress & { kubernetes_services?: IPAddress["kubernetes_services"] }, fallbackServices: IPAddress["kubernetes_services"] = []): IPAddress {
	const hasServices = Object.prototype.hasOwnProperty.call(record, "kubernetes_services");
	const services = hasServices && Array.isArray(record.kubernetes_services) ? record.kubernetes_services : undefined;
	return { ...record, kubernetes_services: services?.length || !fallbackServices.length ? services ?? [] : fallbackServices };
}

export const api = {
	subnets: (requester: Requester) => json<Subnet[]>(requester, "/subnets"),
	sites: (requester: Requester) => json<Site[]>(requester, "/sites"),
	siteStatistics: (requester: Requester) => json<SiteStatistics[]>(requester, "/sites/statistics"),
	ips: async (requester: Requester, subnetId: number) => (await json<Array<IPAddress & { kubernetes_services?: IPAddress["kubernetes_services"] }>>(requester, `/subnets/${subnetId}/ips`)).map((record) => mapIPAddress(record)),
	kubernetesServices: (requester: Requester, subnetId: number) => json<KubernetesServiceObservation[]>(requester, `/subnets/${subnetId}/kubernetes-services`),
	saveSubnet: (requester: Requester, subnet: Partial<Subnet> & Pick<Subnet, "cidr" | "description">) =>
		json<Subnet>(requester, subnet.id ? `/subnets/${subnet.id}` : "/subnets", {
			method: subnet.id ? "PATCH" : "POST",
			headers: { "Content-Type": "application/json" },
			body: JSON.stringify({ cidr: subnet.cidr.trim(), description: subnet.description.trim(), site_id: subnet.site_id || undefined }),
		}),
	deleteSubnet: (requester: Requester, id: number) => requester(`${API_BASE}/subnets/${id}`, { method: "DELETE" }),
	saveSite: (requester: Requester, site: { id?: string; name: string; description: string }) =>
		json<Site>(requester, site.id ? `/sites/${site.id}` : "/sites", {
			method: site.id ? "PATCH" : "POST",
			headers: { "Content-Type": "application/json" },
			body: JSON.stringify({ name: site.name.trim(), description: site.description.trim() }),
		}),
	deleteSite: (requester: Requester, id: string) => requester(`${API_BASE}/sites/${id}`, { method: "DELETE" }),
	saveIp: async (requester: Requester, subnetId: number, existing: IPAddress | undefined, ip: string, hostname: string) =>
		mapIPAddress(await json<IPAddress & { kubernetes_services?: IPAddress["kubernetes_services"] }>(requester, existing ? `/subnets/${subnetId}/ips/${existing.id}` : `/subnets/${subnetId}/ips`, {
			method: existing ? "PATCH" : "POST",
			headers: { "Content-Type": "application/json" },
			body: JSON.stringify(existing ? { hostname: hostname.trim() } : { ip, hostname: hostname.trim() }),
		}), existing?.kubernetes_services),
	deleteIp: (requester: Requester, subnetId: number, id: string) => requester(`${API_BASE}/subnets/${subnetId}/ips/${id}`, { method: "DELETE" }),
	importCSV: (requester: Requester, file: File) => {
		const form = new FormData();
		form.append("file", file);
		return json<ImportResult>(requester, "/import/csv", { method: "POST", body: form });
	},
};
