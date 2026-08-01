import { useEffect, useRef } from "react";

type Props = { title: string; description: string; busy?: boolean; onCancel: () => void; onConfirm: () => void };

export default function ConfirmDialog({ title, description, busy = false, onCancel, onConfirm }: Props) {
	const cancelRef = useRef<HTMLButtonElement>(null);
	const dialogRef = useRef<HTMLElement>(null);
	const originRef = useRef<HTMLElement | null>(document.activeElement instanceof HTMLElement ? document.activeElement : null);
	useEffect(() => {
		cancelRef.current?.focus();
		return () => originRef.current?.focus();
	}, []);
	useEffect(() => {
		const onKeyDown = (event: KeyboardEvent) => {
			if (event.key === "Escape" && !busy) onCancel();
			if (event.key !== "Tab") return;
			const focusable = Array.from(dialogRef.current?.querySelectorAll<HTMLElement>("button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex=\"-1\"])") || []);
			if (!focusable.length) return;
			const first = focusable[0];
			const last = focusable[focusable.length - 1];
			if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus(); }
			else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
		};
		document.addEventListener("keydown", onKeyDown);
		return () => document.removeEventListener("keydown", onKeyDown);
	}, [busy, onCancel]);

	return <div className="modal" role="presentation">
		<div className="modal__backdrop" onClick={() => { if (!busy) onCancel(); }} />
		<section ref={dialogRef} className="modal__content" role="dialog" aria-modal="true" aria-labelledby="confirm-dialog-title">
			<div className="card__header"><p className="eyebrow">Please confirm</p><h2 className="title" id="confirm-dialog-title">{title}</h2><p className="muted">{description}</p></div>
			<div className="modal__actions"><button ref={cancelRef} type="button" className="secondary" disabled={busy} onClick={onCancel}>Cancel</button><button type="button" className="secondary danger" disabled={busy} onClick={onConfirm}>{busy ? "Deleting…" : "Delete"}</button></div>
		</section>
	</div>;
}
