type EnvMap = Record<string, string | undefined>;

const runtimeEnv = ((window as unknown as { __ENV__?: EnvMap }).__ENV__ ?? {}) as EnvMap;
const buildEnv = import.meta.env as EnvMap;

export const getEnv = (key: string, fallback = ""): string => {
	const runtimeValue = runtimeEnv[key];
	if (runtimeValue) {
		return runtimeValue;
	}
	const buildValue = buildEnv[key];
	if (buildValue) {
		return buildValue;
	}
	return fallback;
};
