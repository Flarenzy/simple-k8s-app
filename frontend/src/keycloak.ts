import Keycloak from "keycloak-js";
import { getEnv } from "./env";

const url = getEnv("VITE_KEYCLOAK_URL");
const realm = getEnv("VITE_KEYCLOAK_REALM");
const clientId = getEnv("VITE_KEYCLOAK_CLIENT_ID");

export const keycloakEnabled = Boolean(url && realm && clientId);

const keycloak = keycloakEnabled
	? new Keycloak({
			url,
			realm,
			clientId,
		})
	: null;

let keycloakInitPromise: Promise<boolean> | null = null;
let keycloakInitialized = false;

export const initKeycloak = () => {
	if (!keycloakEnabled || !keycloak) {
		return Promise.resolve(false);
	}
	if (keycloakInitPromise) {
		return keycloakInitPromise;
	}
	if (keycloakInitialized) {
		return Promise.resolve(Boolean(keycloak.authenticated));
	}

	keycloakInitPromise = keycloak
		.init({ onLoad: "login-required", checkLoginIframe: false })
		.then((authenticated) => {
			keycloakInitialized = true;
			return authenticated;
		})
		.catch((err) => {
			keycloakInitPromise = null;
			throw err;
		});

	return keycloakInitPromise;
};

export default keycloak;
