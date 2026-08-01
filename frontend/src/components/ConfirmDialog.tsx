import { useEffect, useRef } from "react";

type Props = { title: string; description: string; busy?: boolean; onCancel: () => void; onConfirm: () => void };

export default function ConfirmDialog({ title, description, busy = false, onCancel, onConfirm }: Props) {
	const cancelRef = useRef<HTMLButtonElement>(null);
	useEffect(() => {
		cancelRef.current?.focus();
		const onKeyDown = (event: KeyboardEvent) => {
			if (event.key === "Escape" && !busy) onCancel();
		};
		document.addEventListener("keydown", onKeyDown);
		return () => document.removeEventListener("keydown", onKeyDown);
	}, [busy, onCancel]);

	return <div className="modal" role="presentation">
		<div className="modal__backdrop" onClick={() => { if (!busy) onCancel(); }} />
		<section className="modal__content" role="dialog" aria-modal="true" aria-labelledby="confirm-dialog-title">
			<div className="card__header"><p className="eyebrow">Please confirm</p><h2 className="title" id="confirm-dialog-title">{title}</h2><p className="muted">{description}</p></div>
			<div className="modal__actions"><button ref={cancelRef} type="button" className="secondary" disabled={busy} onClick={onCancel}>Cancel</button><button type="button" className="secondary danger" disabled={busy} onClick={onConfirm}>{busy ? "Deleting…" : "Delete"}</button></div>
		</section>
	</div>;
}
