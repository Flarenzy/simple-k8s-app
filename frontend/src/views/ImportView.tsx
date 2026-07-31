import { FormEvent, useRef, useState } from "react";
import type { ImportResult } from "../types";

type Props = { onImport: (file: File) => Promise<ImportResult> };

const MAX_IMPORT_FILE_SIZE = 10 * 1024 * 1024;
const MAX_VISIBLE_ERRORS = 200;

function formatFileSize(bytes: number) {
	if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
	return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

export default function ImportView({ onImport }: Props) {
	const inputRef = useRef<HTMLInputElement>(null);
	const [file, setFile] = useState<File | null>(null);
	const [pending, setPending] = useState(false);
	const [result, setResult] = useState<ImportResult | null>(null);
	const [error, setError] = useState<string | null>(null);

	const submit = async (event: FormEvent) => {
		event.preventDefault();
		if (!file) { setError("Choose a CSV file to import."); return; }
		setPending(true); setError(null); setResult(null);
		try { setResult(await onImport(file)); } catch (err) { setError(err instanceof Error ? err.message : "Unable to import CSV"); } finally { setPending(false); }
	};

	const visibleErrors = result?.errors.slice(0, MAX_VISIBLE_ERRORS) ?? [];
	const hiddenErrorCount = (result?.errors.length ?? 0) - visibleErrors.length;

	return <main className="content"><div className="page-heading"><div><p className="eyebrow">Bulk update</p><h1>Import CSV</h1><p className="muted">Create or update sites, subnets, and IP metadata from one file.</p></div></div><div className="import-grid"><section className="card"><div><h2 className="panel__title">Upload inventory</h2><p className="muted">Validation and hierarchy rules stay on the server. Existing records may be updated safely.</p></div><form className="import-form" onSubmit={submit}><label className="file-picker"><span>CSV file</span><input ref={inputRef} type="file" accept=".csv,text/csv" onChange={(event) => { const nextFile = event.target.files?.[0] ?? null; event.currentTarget.value = ""; setError(nextFile && nextFile.size > MAX_IMPORT_FILE_SIZE ? `Choose a CSV file smaller than ${formatFileSize(MAX_IMPORT_FILE_SIZE)}.` : null); setFile(nextFile && nextFile.size <= MAX_IMPORT_FILE_SIZE ? nextFile : null); setResult(null); }} disabled={pending} /><strong>{file?.name ?? "Choose a CSV file"}</strong>{file ? <small>{formatFileSize(file.size)}</small> : <small>Maximum file size: {formatFileSize(MAX_IMPORT_FILE_SIZE)}</small>}</label>{error ? <div className="error" role="alert">{error}</div> : null}<button type="submit" className="primary" disabled={pending || !file}>{pending ? "Uploading…" : "Upload CSV"}</button></form></section><section className="card import-format"><div><h2 className="panel__title">Expected format</h2><p className="muted">Use exactly these columns in the first row:</p></div><code>site,cidr,ip,description</code><p className="muted">Add one IP address per row. Sites and subnets are created when they do not already exist. The description column becomes the IP metadata.</p><pre>{"site,cidr,ip,description\nBelgrade,10.0.0.0/24,10.0.0.10,printer-1"}</pre></section></div>{result ? <section className="card import-result" aria-live="polite"><div className="section-heading"><div><p className="eyebrow">Import complete</p><h2 className="panel__title">Inventory refresh finished</h2></div><span className={result.failed ? "result-status result-status--warning" : "result-status"}>{result.failed ? "Completed with errors" : "Successful"}</span></div><div className="import-stats"><div><strong>{result.processed.toLocaleString()}</strong><span>Processed</span></div><div><strong>{result.created.toLocaleString()}</strong><span>Created</span></div><div><strong>{result.updated.toLocaleString()}</strong><span>Updated</span></div><div><strong>{result.failed.toLocaleString()}</strong><span>Failed</span></div></div>{visibleErrors.length ? <div className="import-errors"><h3>Rows needing attention</h3><ul>{visibleErrors.map((item, index) => <li key={`${item.row}-${index}`}><strong>Row {item.row}:</strong> {item.message}</li>)}</ul>{hiddenErrorCount > 0 ? <p>Showing the first {MAX_VISIBLE_ERRORS} errors ({hiddenErrorCount} more not shown).</p> : null}</div> : <p className="success">All rows passed backend validation.</p>}</section> : null}</main>;
}
