import Keycloak from "keycloak-js";
import { getEnv } from "./env";

const keycloak = new Keycloak({
	url: getEnv("VITE_KEYCLOAK_URL", "http://localhost:8080"),
	realm: getEnv("VITE_KEYCLOAK_REALM", "ipam"),
	clientId: getEnv("VITE_KEYCLOAK_CLIENT_ID", "ipam-fe"),
});

export default keycloak;
