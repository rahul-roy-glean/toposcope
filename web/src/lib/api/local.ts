import { HttpAPI } from "./http";

const BASE_URL = process.env.NEXT_PUBLIC_API_BASE_URL || "http://localhost:7700";

export class LocalAPI extends HttpAPI {
  constructor() {
    super(BASE_URL);
  }
}
