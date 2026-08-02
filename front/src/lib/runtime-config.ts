import { createContext, useContext } from "react";

export interface RuntimeConfig {
  replay_origin: string;
}

export const RuntimeConfigContext = createContext<RuntimeConfig | null>(null);

export async function loadRuntimeConfig(): Promise<RuntimeConfig> {
  const response = await fetch("/api/config");
  if (!response.ok) {
    throw new Error(`Failed to load runtime configuration: ${response.status}`);
  }

  const config = (await response.json()) as RuntimeConfig;
  const replayOrigin = new URL(config.replay_origin);
  if (!/^https?:$/.test(replayOrigin.protocol)) {
    throw new Error("Runtime configuration contains an invalid replay origin");
  }

  return { replay_origin: replayOrigin.origin };
}

export function useRuntimeConfig(): RuntimeConfig {
  const config = useContext(RuntimeConfigContext);
  if (!config) {
    throw new Error("Runtime configuration is not available");
  }
  return config;
}
