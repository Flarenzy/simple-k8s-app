/**
 * @param {string} cidr
 * @returns {{ first: number; count: number } | null}
 */
export function parseUsableIPv4Cidr(cidr) {
	const [address, maskText] = cidr.split("/");
	const mask = Number(maskText);
	const octets = address?.split(".").map(Number);
	if (!address || !Number.isInteger(mask) || mask < 0 || mask > 32 || !octets || octets.length !== 4 || octets.some((part) => !Number.isInteger(part) || part < 0 || part > 255)) return null;

	const network = ((((octets[0] << 24) | (octets[1] << 16) | (octets[2] << 8) | octets[3]) >>> 0) & (mask === 0 ? 0 : (0xffffffff << (32 - mask)) >>> 0)) >>> 0;
	const addressCount = mask === 32 ? 1 : 2 ** (32 - mask);
	const reservedCount = mask < 31 ? 2 : 0;
	return {
		first: (network + (reservedCount ? 1 : 0)) >>> 0,
		count: addressCount - reservedCount,
	};
}

/**
 * @param {number} value
 * @returns {string}
 */
export function formatIPv4(value) {
	return `${value >>> 24 & 255}.${value >>> 16 & 255}.${value >>> 8 & 255}.${value & 255}`;
}
