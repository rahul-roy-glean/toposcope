import { HttpAPI } from "./http";

export class HostedAPI extends HttpAPI {
  constructor(baseUrl?: string) {
    const url = baseUrl || process.env.NEXT_PUBLIC_API_BASE_URL || "";
    if (!url) {
      throw new Error("NEXT_PUBLIC_API_BASE_URL must be set for hosted mode");
    }
    super(url);
  }
}
