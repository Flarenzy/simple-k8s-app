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
