import type { ToposcopeAPI } from "./client";
import type { Repository, ScoreResult, Snapshot, Subgraph, ScoreHistory, PackageGraph, EgoGraph, PathResult } from "@/lib/types";
import { mockRepos, mockScores, mockSnapshot, mockScoreHistory } from "./mock-data";

export class MockAPI implements ToposcopeAPI {
  private delay(ms: number = 200): Promise<void> {
    return new Promise((resolve) => setTimeout(resolve, ms));
  }

  async getRepos(): Promise<Repository[]> {
    await this.delay();
    return mockRepos;
  }

  async getScores(repoId: string): Promise<ScoreResult[]> {
    await this.delay();
    return mockScores[repoId] ?? [];
  }

  async getPRImpact(repoId: string, prNumber: number): Promise<ScoreResult> {
    await this.delay(300);
    const scores = mockScores[repoId] ?? [];
    const match = scores.find((s) => s.pr_number === prNumber);
    if (match) return match;
    return scores[0] ?? ({} as ScoreResult);
  }

  async getSnapshot(snapshotId: string): Promise<Snapshot> {
    await this.delay(300);
    return { ...mockSnapshot, id: snapshotId };
  }

  async getSubgraph(snapshotId: string, roots: string[], depth: number): Promise<Subgraph> {
    await this.delay(300);
    if (roots.length === 0 || depth <= 0) {
      return { nodes: mockSnapshot.nodes, edges: mockSnapshot.edges };
    }

    const included = new Set<string>();
    const queue = [...roots];
    let currentDepth = 0;

    while (queue.length > 0 && currentDepth < depth) {
      const batch = [...queue];
      queue.length = 0;
      for (const key of batch) {
        if (included.has(key)) continue;
        included.add(key);
        for (const edge of mockSnapshot.edges) {
          if (edge.from === key && !included.has(edge.to)) {
            queue.push(edge.to);
          }
          if (edge.to === key && !included.has(edge.from)) {
            queue.push(edge.from);
          }
        }
      }
      currentDepth++;
    }

    const nodes: Record<string, typeof mockSnapshot.nodes[string]> = {};
    for (const key of included) {
      if (mockSnapshot.nodes[key]) {
        nodes[key] = mockSnapshot.nodes[key];
      }
    }
    const edges = mockSnapshot.edges.filter(
      (e) => included.has(e.from) && included.has(e.to)
    );

    return { nodes, edges };
  }

  async getScoreHistory(repoId: string): Promise<ScoreHistory[]> {
    await this.delay();
    return mockScoreHistory[repoId] ?? [];
  }

  async getPackages(_snapshotId: string, _opts?: { hideTests?: boolean; hideExternal?: boolean; minEdgeWeight?: number }): Promise<PackageGraph> {
    await this.delay();
    return { nodes: {}, edges: [], truncated: false };
  }

  async getEgoGraph(_snapshotId: string, _target: string, _opts?: { depth?: number; direction?: "deps" | "rdeps" | "both" }): Promise<EgoGraph> {
    await this.delay();
    return { nodes: {}, edges: [], truncated: false };
  }

  async getPath(_snapshotId: string, from: string, to: string, _maxPaths?: number): Promise<PathResult> {
    await this.delay();
    return { paths: [], nodes: {}, edges: [], from, to, path_length: 0 };
  }

  async getScore(_repoId: string, _scoreId: string): Promise<ScoreResult> {
    await this.delay();
    return {} as ScoreResult;
  }
}
