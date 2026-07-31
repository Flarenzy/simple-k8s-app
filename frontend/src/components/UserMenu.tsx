import { useEffect, useRef, useState } from "react";

type Props = { username: string; onChangePassword: () => void; onLogout: () => void };

export default function UserMenu({ username, onChangePassword, onLogout }: Props) {
	const [open, setOpen] = useState(false);
	const ref = useRef<HTMLDivElement>(null);
	useEffect(() => {
		const close = (event: MouseEvent) => { if (!ref.current?.contains(event.target as Node)) setOpen(false); };
		document.addEventListener("click", close);
		return () => document.removeEventListener("click", close);
	}, []);
	return <div className="user-menu" ref={ref}>
		<button type="button" className="user-menu__trigger" onClick={() => setOpen((value) => !value)} aria-expanded={open} aria-haspopup="menu">
			<span className="avatar">{username.charAt(0).toUpperCase()}</span><span>{username}</span><span aria-hidden="true">⌄</span>
		</button>
		{open ? <div className="user-menu__content" role="menu">
			<button type="button" onClick={() => { setOpen(false); onChangePassword(); }}>Change password</button><button type="button" onClick={() => { setOpen(false); onLogout(); }}>Log out</button>
		</div> : null}
	</div>;
}
