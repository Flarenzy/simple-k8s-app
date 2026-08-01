import { useEffect, useRef, useState } from "react";

type Props = {
	label: string;
	onEdit?: () => void;
	onDelete?: () => void;
};

export default function ActionMenu({ label, onEdit, onDelete }: Props) {
	const [open, setOpen] = useState(false);
	const triggerRef = useRef<HTMLButtonElement>(null);
	const menuRef = useRef<HTMLDivElement>(null);
	const itemRefs = useRef<HTMLButtonElement[]>([]);

	useEffect(() => {
		if (open) window.setTimeout(() => itemRefs.current[0]?.focus(), 0);
	}, [open]);

	useEffect(() => {
		if (!open) return;
		const onDocumentPointerDown = (event: PointerEvent) => {
			if (!menuRef.current?.contains(event.target as Node) && !triggerRef.current?.contains(event.target as Node)) setOpen(false);
		};
		const onKeyDown = (event: KeyboardEvent) => {
			if (event.key === "Escape") {
				setOpen(false);
				triggerRef.current?.focus();
			}
			if (!menuRef.current?.contains(document.activeElement)) return;
			const items = itemRefs.current.filter(Boolean);
			const index = items.indexOf(document.activeElement as HTMLButtonElement);
			if (event.key === "ArrowDown" || event.key === "ArrowUp") {
				event.preventDefault();
				items[(index + (event.key === "ArrowDown" ? 1 : items.length - 1)) % items.length]?.focus();
			}
			if (event.key === "Home") { event.preventDefault(); items[0]?.focus(); }
			if (event.key === "End") { event.preventDefault(); items[items.length - 1]?.focus(); }
		};
		document.addEventListener("pointerdown", onDocumentPointerDown);
		document.addEventListener("keydown", onKeyDown);
		return () => {
			document.removeEventListener("pointerdown", onDocumentPointerDown);
			document.removeEventListener("keydown", onKeyDown);
		};
	}, [open]);

	const choose = (action?: () => void) => {
		setOpen(false);
		triggerRef.current?.focus();
		action?.();
	};

	return <div className="action-menu">
		<button ref={triggerRef} type="button" className="secondary action-menu__trigger" aria-label={`${label} actions`} aria-haspopup="menu" aria-expanded={open} onClick={() => setOpen((value) => !value)}>
			<span aria-hidden="true">•••</span>
		</button>
		{open ? <div ref={menuRef} className="action-menu__content" role="menu" aria-label={`${label} actions`}>
			{onEdit ? <button ref={(element) => { if (element) itemRefs.current[0] = element; }} type="button" role="menuitem" onClick={() => choose(onEdit)}>Edit</button> : null}
			{onDelete ? <button ref={(element) => { if (element) itemRefs.current[onEdit ? 1 : 0] = element; }} type="button" role="menuitem" className="action-menu__delete" onClick={() => choose(onDelete)}>Delete</button> : null}
		</div> : null}
	</div>;
}
