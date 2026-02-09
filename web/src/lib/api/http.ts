import type { ToposcopeAPI } from "./client";
import type { Repository, ScoreResult, Snapshot, Subgraph, ScoreHistory, PackageGraph, EgoGraph, PathResult } from "@/lib/types";

export class HttpAPI implements ToposcopeAPI {
  constructor(private baseUrl: string) {}

  private async fetchJSON<T>(path: string): Promise<T> {
    const res = await fetch(`${this.baseUrl}${path}`, {
      headers: { "Content-Type": "application/json" },
    });
    if (!res.ok) {
      throw new Error(`API error: ${res.status} ${res.statusText}`);
    }
    return res.json() as Promise<T>;
  }

  async getRepos(): Promise<Repository[]> {
    return this.fetchJSON("/api/repos");
  }

  async getScores(repoId: string): Promise<ScoreResult[]> {
    return this.fetchJSON(`/api/repos/${repoId}/scores`);
  }

  async getPRImpact(repoId: string, prNumber: number): Promise<ScoreResult> {
    return this.fetchJSON(`/api/repos/${repoId}/prs/${prNumber}/impact`);
  }

  async getSnapshot(snapshotId: string): Promise<Snapshot> {
    return this.fetchJSON(`/api/snapshots/${snapshotId}`);
  }

  async getSubgraph(snapshotId: string, roots: string[], depth: number): Promise<Subgraph> {
    const params = new URLSearchParams();
    for (const r of roots) params.append("root", r);
    params.set("depth", String(depth));
    return this.fetchJSON(`/api/snapshots/${snapshotId}/subgraph?${params}`);
  }

  async getScoreHistory(repoId: string): Promise<ScoreHistory[]> {
    return this.fetchJSON(`/api/repos/${repoId}/history`);
  }

  async getPackages(snapshotId: string, opts?: { hideTests?: boolean; hideExternal?: boolean; minEdgeWeight?: number }): Promise<PackageGraph> {
    const params = new URLSearchParams();
    if (opts?.hideTests) params.set("hide_tests", "true");
    if (opts?.hideExternal) params.set("hide_external", "true");
    if (opts?.minEdgeWeight) params.set("min_edge_weight", String(opts.minEdgeWeight));
    const qs = params.toString();
    return this.fetchJSON(`/api/snapshots/${snapshotId}/packages${qs ? `?${qs}` : ""}`);
  }

  async getEgoGraph(snapshotId: string, target: string, opts?: { depth?: number; direction?: "deps" | "rdeps" | "both" }): Promise<EgoGraph> {
    const params = new URLSearchParams();
    params.set("target", target);
    if (opts?.depth) params.set("depth", String(opts.depth));
    if (opts?.direction) params.set("direction", opts.direction);
    return this.fetchJSON(`/api/snapshots/${snapshotId}/ego?${params}`);
  }

  async getPath(snapshotId: string, from: string, to: string, maxPaths?: number): Promise<PathResult> {
    const params = new URLSearchParams();
    params.set("from", from);
    params.set("to", to);
    if (maxPaths) params.set("max_paths", String(maxPaths));
    return this.fetchJSON(`/api/snapshots/${snapshotId}/path?${params}`);
  }

  async getScore(repoId: string, scoreId: string): Promise<ScoreResult> {
    return this.fetchJSON(`/api/repos/${repoId}/scores/${scoreId}`);
  }
}
