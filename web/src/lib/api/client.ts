import type { Repository, ScoreResult, Snapshot, Subgraph, ScoreHistory, PackageGraph, EgoGraph, PathResult } from "@/lib/types";

export interface ToposcopeAPI {
  getRepos(): Promise<Repository[]>;
  getScores(repoId: string): Promise<ScoreResult[]>;
  getPRImpact(repoId: string, prNumber: number): Promise<ScoreResult>;
  getSnapshot(snapshotId: string): Promise<Snapshot>;
  getSubgraph(snapshotId: string, roots: string[], depth: number): Promise<Subgraph>;
  getScoreHistory(repoId: string): Promise<ScoreHistory[]>;
  getPackages(snapshotId: string, opts?: { hideTests?: boolean; hideExternal?: boolean; minEdgeWeight?: number }): Promise<PackageGraph>;
  getEgoGraph(snapshotId: string, target: string, opts?: { depth?: number; direction?: "deps" | "rdeps" | "both" }): Promise<EgoGraph>;
  getPath(snapshotId: string, from: string, to: string, maxPaths?: number): Promise<PathResult>;
  getScore(repoId: string, scoreId: string): Promise<ScoreResult>;
}

export type APIMode = "local" | "hosted" | "mock";

export function getAPIMode(): APIMode {
  const mode = process.env.NEXT_PUBLIC_API_MODE;
  if (mode === "local" || mode === "hosted" || mode === "mock") {
    return mode;
  }
  return "mock";
}
