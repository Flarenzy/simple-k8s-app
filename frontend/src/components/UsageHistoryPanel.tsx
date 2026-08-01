import { useEffect, useMemo, useState, type FormEvent } from "react";
import { api, type Requester } from "../api";
import type { ReportingCadence, ReportingSettings, SubnetUsageHistory, UsageRange } from "../types";

const rangeOptions: { value: UsageRange; label: string; days: number }[] = [
	{ value: "24h", label: "24 hours", days: 1 },
	{ value: "7d", label: "7 days", days: 7 },
	{ value: "30d", label: "30 days", days: 30 },
	{ value: "90d", label: "90 days", days: 90 },
	{ value: "180d", label: "180 days", days: 180 },
];

const cadenceMilliseconds: Record<ReportingCadence, number> = {
	hourly: 60 * 60 * 1000,
	daily: 24 * 60 * 60 * 1000,
	weekly: 7 * 24 * 60 * 60 * 1000,
};

type ChartPoint = { x: number; y: number; used: number; capturedAt: string };

function chartSegments(history: SubnetUsageHistory): { segments: ChartPoint[][]; capacity: number } {
	const width = 640;
	const height = 160;
	const from = new Date(history.from).getTime();
	const to = new Date(history.to).getTime();
	const duration = Math.max(to - from, 1);
	const capacity = Math.max(...history.points.map((point) => point.total_ips), 1);
	const points = history.points.map((point) => ({
		x: 44 + (new Date(point.captured_at).getTime() - from) / duration * width,
		y: 16 + (1 - point.used_ips / capacity) * height,
		used: point.used_ips,
		capturedAt: point.captured_at,
	}));
	const segments: ChartPoint[][] = [];
	for (const point of points) {
		const segment = segments[segments.length - 1];
		const previous = segment?.[segment.length - 1];
		if (!previous || new Date(point.capturedAt).getTime() - new Date(previous.capturedAt).getTime() > cadenceMilliseconds[history.cadence] * 1.5) {
			segments.push([point]);
		} else {
			segment.push(point);
		}
	}
	return { segments, capacity };
}

function UsageChart({ history }: { history: SubnetUsageHistory }) {
	const { segments, capacity } = useMemo(() => chartSegments(history), [history]);
	const latest = history.points[history.points.length - 1];
	return <div className="usage-chart"><div className="usage-chart__summary"><div><span className="field-label">Latest snapshot</span><strong>{latest ? `${latest.used_ips.toLocaleString()} used` : "No snapshot"}</strong></div><div><span className="field-label">Capacity</span><strong>{latest?.total_ips.toLocaleString() ?? "—"}</strong></div><div><span className="field-label">Samples</span><strong>{history.points.length.toLocaleString()}</strong></div></div>{history.points.length ? <svg viewBox="0 0 720 220" role="img" aria-label={`Subnet usage history with ${history.points.length} recorded snapshots`}><line className="usage-chart__grid" x1="44" x2="684" y1="16" y2="16" /><line className="usage-chart__grid" x1="44" x2="684" y1="96" y2="96" /><line className="usage-chart__axis" x1="44" x2="684" y1="176" y2="176" /><text x="4" y="21">{capacity.toLocaleString()}</text><text x="26" y="181">0</text><text x="44" y="207">{new Date(history.from).toLocaleDateString()}</text><text x="684" y="207" textAnchor="end">{new Date(history.to).toLocaleDateString()}</text>{segments.map((segment, index) => segment.length > 1 ? <polyline className="usage-chart__line" key={index} points={segment.map((point) => `${point.x},${point.y}`).join(" ")} /> : null)}{segments.flat().map((point) => <circle className="usage-chart__point" key={point.capturedAt} cx={point.x} cy={point.y} r="4"><title>{new Date(point.capturedAt).toLocaleString()}: {point.used.toLocaleString()} used</title></circle>)}</svg> : <div className="usage-chart__empty"><strong>No snapshots in this range</strong><p className="muted">Reporting captures current inventory periodically; it does not backfill history from existing rows.</p></div>}<p className="usage-chart__note">Only recorded snapshots are plotted. Missed intervals remain unconnected.</p></div>;
}

type Props = { subnetId: number; cidr: string; requester: Requester; canEdit: boolean };

export default function UsageHistoryPanel({ subnetId, cidr, requester, canEdit }: Props) {
	const ipv4 = !cidr.includes(":");
	const [settings, setSettings] = useState<ReportingSettings | null>(null);
	const [history, setHistory] = useState<SubnetUsageHistory | null>(null);
	const [range, setRange] = useState<UsageRange>("7d");
	const [cadence, setCadence] = useState<ReportingCadence>("hourly");
	const [retention, setRetention] = useState(30);
	const [loading, setLoading] = useState(true);
	const [saving, setSaving] = useState(false);
	const [error, setError] = useState<string | null>(null);

	useEffect(() => {
		let cancelled = false;
		setLoading(true);
		setError(null);
		const settingsRequest = api.reportingSettings(requester);
		const historyRequest = ipv4 ? api.usageHistory(requester, subnetId, range) : Promise.resolve(null);
		void Promise.all([settingsRequest, historyRequest]).then(([nextSettings, nextHistory]) => {
			if (cancelled) return;
			setSettings(nextSettings);
			setCadence(nextSettings.cadence);
			setRetention(nextSettings.retention_days);
			setHistory(nextHistory);
		}).catch((err) => { if (!cancelled) setError(err instanceof Error ? err.message : "Unable to load reporting history"); }).finally(() => { if (!cancelled) setLoading(false); });
		return () => { cancelled = true; };
	}, [ipv4, range, requester, subnetId]);

	const availableRanges = rangeOptions.filter((option) => option.days <= (settings?.retention_days ?? retention));
	const saveSettings = async (event: FormEvent) => {
		event.preventDefault();
		setSaving(true);
		setError(null);
		try {
			const nextSettings = await api.updateReportingSettings(requester, { cadence, retention_days: retention });
			setSettings(nextSettings);
			const retainedRanges = rangeOptions.filter((option) => option.days <= nextSettings.retention_days);
			const nextRange = rangeOptions.find((option) => option.value === range && option.days <= nextSettings.retention_days)?.value ?? retainedRanges[retainedRanges.length - 1]?.value ?? "24h";
			if (nextRange !== range) setRange(nextRange);
			else if (ipv4) setHistory(await api.usageHistory(requester, subnetId, range));
		} catch (err) {
			setError(err instanceof Error ? err.message : "Unable to update reporting settings");
		} finally {
			setSaving(false);
		}
	};

	return <section className="card reporting-panel"><div className="section-heading"><div><p className="eyebrow">Periodic snapshots</p><h2>Subnet usage</h2><p className="muted">Address allocations recorded over time. Kubernetes observations are excluded.</p></div>{settings?.last_snapshot_at ? <span className="pill" title={new Date(settings.last_snapshot_at).toLocaleString()}>Last snapshot {new Date(settings.last_snapshot_at).toLocaleDateString()}</span> : <span className="pill pill--muted">Awaiting first snapshot</span>}</div>{error ? <div className="error" role="alert">{error}</div> : null}<div className="reporting-layout"><div className="reporting-history"><div className="range-picker" aria-label="Usage history range">{availableRanges.map((option) => <button className={range === option.value ? "secondary range-picker__active" : "secondary"} type="button" key={option.value} onClick={() => setRange(option.value)}>{option.label}</button>)}</div>{!ipv4 ? <div className="usage-chart__empty"><strong>IPv6 reporting is not available yet</strong><p className="muted">This MVP records IPv4 subnet usage only.</p></div> : loading || !history ? <div className="usage-chart__empty" aria-live="polite"><span className="loading-indicator" /><p className="muted">Loading usage snapshots…</p></div> : <UsageChart history={history} />}</div><form className="reporting-settings" onSubmit={saveSettings}><div><span className="field-label">Snapshot settings</span><p className="muted">Applies to all IPv4 subnets.</p></div><label className="field"><span>Cadence</span><select value={cadence} disabled={!canEdit || saving} onChange={(event) => setCadence(event.target.value as ReportingCadence)}><option value="hourly">Hourly</option><option value="daily">Daily</option><option value="weekly">Weekly</option></select></label><label className="field"><span>Retention (days)</span><input type="number" min="1" max="180" value={retention} disabled={!canEdit || saving} onChange={(event) => setRetention(Number(event.target.value))} /></label>{canEdit ? <button className="primary" disabled={saving || retention < 1 || retention > 180}>{saving ? "Saving…" : "Save settings"}</button> : <p className="muted">Read-only access</p>}</form></div></section>;
}
