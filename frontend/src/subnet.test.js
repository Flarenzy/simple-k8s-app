import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { formatIPv4, parseUsableIPv4Cidr } from "./subnet.js";

describe("usable IPv4 subnet ranges", () => {
	for (const testCase of [
		{ cidr: "10.0.0.0/24", first: "10.0.0.1", count: 254 },
		{ cidr: "10.0.0.0/31", first: "10.0.0.0", count: 2 },
		{ cidr: "10.0.0.1/32", first: "10.0.0.1", count: 1 },
	]) {
		it(`uses the documented capacity for ${testCase.cidr}`, () => {
			const range = parseUsableIPv4Cidr(testCase.cidr);
			assert.ok(range);
			assert.equal(range.count, testCase.count);
			assert.equal(formatIPv4(range.first), testCase.first);
		});
	}

	it("rejects malformed and IPv6 CIDRs", () => {
		assert.equal(parseUsableIPv4Cidr("not-a-cidr"), null);
		assert.equal(parseUsableIPv4Cidr("2001:db8::/64"), null);
	});
});
