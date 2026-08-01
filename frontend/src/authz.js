/** @typedef {"admin" | "read-only" | "editor"} ApplicationRole */
/** @typedef {{ canCreate: boolean; canEdit: boolean; canDelete: boolean }} Capabilities */
/** @typedef {{ resource_access?: Record<string, { roles?: unknown[] }> }} TokenClaims */

/** @type {Set<ApplicationRole>} */
const applicationRoles = new Set(["admin", "read-only", "editor"]);

/**
 * @param {TokenClaims | undefined} claims
 * @param {string} clientID
 * @returns {ApplicationRole[]}
 */
export function rolesFromToken(claims, clientID) {
	const values = claims?.resource_access?.[clientID]?.roles;
	if (!Array.isArray(values)) return [];

	/** @type {ApplicationRole[]} */
	const roles = [];
	for (const value of values) {
		if (typeof value === "string" && applicationRoles.has(/** @type {ApplicationRole} */ (value))) {
			roles.push(/** @type {ApplicationRole} */ (value));
		}
	}
	return roles;
}

/**
 * @param {ApplicationRole[]} roles
 * @returns {Capabilities}
 */
export function capabilitiesForRoles(roles) {
	const admin = roles.includes("admin");
	const editor = roles.includes("editor");
	return {
		canCreate: admin || editor,
		canEdit: admin || editor,
		canDelete: admin,
	};
}

/**
 * @param {ApplicationRole[]} roles
 * @returns {string}
 */
export function roleLabel(roles) {
	if (roles.includes("admin")) return "Admin";
	if (roles.includes("editor")) return "Editor";
	if (roles.includes("read-only")) return "Read only";
	return "No application role";
}

/** @type {ApplicationRole[]} */
export const localAdminRoles = ["admin"];
