import { RuntimeConfigContext, type RuntimeConfig } from "@/lib/runtime-config";
import type { ReactNode } from "react";

export function RuntimeConfigProvider({
  config,
  children,
}: {
  config: RuntimeConfig;
  children: ReactNode;
}) {
  return (
    <RuntimeConfigContext.Provider value={config}>
      {children}
    </RuntimeConfigContext.Provider>
  );
}
