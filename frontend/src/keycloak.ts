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

export default keycloak;
