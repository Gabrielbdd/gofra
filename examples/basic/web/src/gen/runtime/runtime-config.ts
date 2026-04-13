import type { RuntimeConfig } from "./runtime_config_pb";

type RuntimeWindow = Window & {
  __GOFRA_CONFIG__?: unknown;
};

export function isRuntimeConfig(value: unknown): value is RuntimeConfig {
  if (typeof value !== "object" || value === null) {
    return false;
  }

  const candidate = value as Partial<RuntimeConfig>;
  return typeof candidate.auth === "object" && candidate.auth !== null;
}

export function validateRuntimeConfig(value: unknown): RuntimeConfig {
  if (!isRuntimeConfig(value)) {
    throw new Error("missing or invalid runtime config");
  }

  return Object.freeze(value as RuntimeConfig);
}

export function loadRuntimeConfig(): RuntimeConfig {
  return validateRuntimeConfig((window as RuntimeWindow).__GOFRA_CONFIG__);
}

export const runtimeConfig = loadRuntimeConfig();
