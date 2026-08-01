export type View = "dashboard" | "subnets" | "subnet" | "sites" | "import";

export type Subnet = {
	id: number;
	cidr: string;
	site_id?: string;
	used_ips: number;
	total_ips: number;
	description: string;
	created_at: string;
	updated_at: string;
};

export type IPAddress = {
	id: string;
	ip: string;
	hostname: string;
	subnet_id: number;
	created_at: string;
	updated_at: string;
	kubernetes_services: KubernetesService[];
};

export type KubernetesServiceStatus = "matched" | "unmatched" | "ambiguous" | "no_usable_ip";

export type KubernetesServiceAddress = {
	ip: string;
	kind: string;
	match_status?: KubernetesServiceStatus;
};

export type KubernetesServicePort = {
	name?: string;
	protocol: string;
	port: number;
	target_port?: string | number;
	app_protocol?: string;
};

export type KubernetesService = {
	source: { key: string; name: string };
	uid: string;
	name: string;
	namespace: string;
	type: string;
	dns_name: string;
	matched_addresses: KubernetesServiceAddress[];
	ports: KubernetesServicePort[];
	observed_at: string;
	status?: KubernetesServiceStatus;
};

export type KubernetesServiceObservationAddress = KubernetesServiceAddress & {
	ip_mode?: string;
	match_count?: number;
	matched_ip_address_id?: string;
	matched_subnet_id?: number;
};

export type KubernetesServiceObservation = Omit<KubernetesService, "matched_addresses"> & {
	external_name?: string;
	match_status: KubernetesServiceStatus;
	addresses: KubernetesServiceObservationAddress[];
	hostnames?: { kind: string; hostname: string }[];
};

export type KubernetesServiceSummary = {
	count: number;
	statuses: Partial<Record<KubernetesServiceStatus, number>>;
	state?: "loading" | "ready" | "unavailable";
};

export type Site = {
	id: string;
	name: string;
	description: string;
	created_at: string;
	updated_at: string;
};

export type SiteStatistics = Site & {
	subnet_count: number;
	used_ip_count: number;
	total_ip_count: number;
	free_ip_count: number;
};

export type SubnetUsage = { used: number; total: number };

export type ImportRowError = { row: number; message: string };

export type ImportResult = {
	processed: number;
	created: number;
	updated: number;
	failed: number;
	errors: ImportRowError[];
};
