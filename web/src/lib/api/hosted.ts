import { HttpAPI } from "./http";

export class HostedAPI extends HttpAPI {
  constructor(baseUrl?: string) {
    // Empty string means same-origin (browser will use relative URLs).
    super(baseUrl ?? process.env.NEXT_PUBLIC_API_BASE_URL ?? "");
  }
}
