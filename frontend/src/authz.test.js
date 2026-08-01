import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { capabilitiesForRoles, roleLabel, rolesFromToken } from "./authz.js";

describe("application authorization", () => {
	it("extracts only roles for the configured API client", () => {
		assert.deepEqual(rolesFromToken({
			resource_access: {
				"ipam-api": { roles: ["read-only", "editor", "unknown"] },
				other: { roles: ["admin"] },
			},
		}, "ipam-api"), ["read-only", "editor"]);
	});

	for (const testCase of [
		{ roles: ["admin"], capabilities: { canCreate: true, canEdit: true, canDelete: true }, label: "Admin" },
		{ roles: ["editor"], capabilities: { canCreate: true, canEdit: true, canDelete: false }, label: "Editor" },
		{ roles: ["read-only"], capabilities: { canCreate: false, canEdit: false, canDelete: false }, label: "Read only" },
		{ roles: [], capabilities: { canCreate: false, canEdit: false, canDelete: false }, label: "No application role" },
	]) {
		it(`maps ${JSON.stringify(testCase.roles)} to capabilities`, () => {
			assert.deepEqual(capabilitiesForRoles(testCase.roles), testCase.capabilities);
			assert.equal(roleLabel(testCase.roles), testCase.label);
		});
	}
});
