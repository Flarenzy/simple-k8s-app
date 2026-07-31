import type { View } from "../types";
import UserMenu from "./UserMenu";

type Props = { view: View; username: string; authenticated: boolean; onNavigate: (view: View) => void; onChangePassword: () => void; onLogout: () => void };

export default function AppHeader({ view, username, authenticated, onNavigate, onChangePassword, onLogout }: Props) {
	return <header className="app-header">
		<div className="brand"><div className="brand__mark">IP</div><div><strong>IPAM</strong><span>Network inventory</span></div></div>
		<nav className="nav" aria-label="Primary navigation">
			{(["dashboard", "subnets", "sites", "import"] as const).map((item) => <button key={item} type="button" className={view === item ? "nav__item nav__item--active" : "nav__item"} aria-current={view === item ? "page" : undefined} onClick={() => onNavigate(item)}>{item[0].toUpperCase() + item.slice(1)}</button>)}
		</nav>
		{authenticated ? <UserMenu username={username} onChangePassword={onChangePassword} onLogout={onLogout} /> : <span className="user-fallback">Local mode</span>}
	</header>;
}
