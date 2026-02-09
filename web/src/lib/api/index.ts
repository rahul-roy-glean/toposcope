import type { ToposcopeAPI } from "./client";
import { getAPIMode } from "./client";

let _api: ToposcopeAPI | null = null;

export async function getAPI(): Promise<ToposcopeAPI> {
  if (_api) return _api;

  const mode = getAPIMode();

  switch (mode) {
    case "local": {
      const { LocalAPI } = await import("./local");
      _api = new LocalAPI();
      break;
    }
    case "hosted": {
      const { HostedAPI } = await import("./hosted");
      _api = new HostedAPI();
      break;
    }
    case "mock":
    default: {
      const { MockAPI } = await import("./mock");
      _api = new MockAPI();
      break;
    }
  }

  return _api;
}

export type { ToposcopeAPI } from "./client";
