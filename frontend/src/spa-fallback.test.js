import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { describe, it } from "node:test";

const nginxConfig = readFileSync(new URL("../../deploy/docker/nginx.fe.conf", import.meta.url), "utf8");
const dockerfile = readFileSync(new URL("../../deploy/docker/Dockerfile.fe", import.meta.url), "utf8");

describe("frontend container routing", () => {
	it("falls back to the SPA entry point for browser routes", () => {
		assert.match(nginxConfig, /location \/\s*\{[^}]*try_files \$uri \$uri\/ \/index\.html;/s);
	});

	it("installs the SPA-aware NGINX configuration", () => {
		assert.match(dockerfile, /COPY deploy\/docker\/nginx\.fe\.conf \/etc\/nginx\/conf\.d\/default\.conf/);
	});
});
