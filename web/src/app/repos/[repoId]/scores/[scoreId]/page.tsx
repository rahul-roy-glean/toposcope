"use client";

import { useEffect, useState, useMemo, useCallback } from "react";
import { useParams } from "next/navigation";
import { ArrowLeft, Plus, Minus, Target, GitCommit, Network } from "lucide-react";
import Link from "next/link";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { GradeBadge } from "@/components/scoring/grade-badge";
import { Progress } from "@/components/ui/progress";
import { ScoreBreakdown } from "@/components/scoring/score-breakdown";
import { HotspotList } from "@/components/scoring/hotspot-list";
import { Suggestions } from "@/components/scoring/suggestions";
import { DependencyGraph } from "@/components/graph/dependency-graph";
import { NodeDetail } from "@/components/graph/node-detail";
import { getAPI } from "@/lib/api";
import type { ScoreResult, Node, Edge } from "@/lib/types";

export default function ScoreDetailPage() {
  const params = useParams<{ repoId: string; scoreId: string }>();
  const [score, setScore] = useState<ScoreResult | null>(null);
  const [graphNodes, setGraphNodes] = useState<Record<string, Node>>({});
  const [graphEdges, setGraphEdges] = useState<Edge[]>([]);
  const [selectedNode, setSelectedNode] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Transitive impact state
  const [impactDepth, setImpactDepth] = useState(2);
  const [impactDirection, setImpactDirection] = useState<"deps" | "rdeps" | "both">("both");
  const [impactNodes, setImpactNodes] = useState<Record<string, Node>>({});
  const [impactEdges, setImpactEdges] = useState<Edge[]>([]);
  const [impactLoading, setImpactLoading] = useState(false);
  const [impactTruncated, setImpactTruncated] = useState(false);
  const [selectedImpactNode, setSelectedImpactNode] = useState<string | null>(null);
  const [impactTarget, setImpactTarget] = useState<string | null>(null);

  useEffect(() => {
    async function load() {
      try {
        const api = await getAPI();
        const s = await api.getScore(params.repoId, params.scoreId);
        setScore(s);

        // Load subgraph around hotspot nodes from the head snapshot
        if (s.head_commit && s.hotspots?.length > 0) {
          const hotspotKeys = s.hotspots.map((h) => h.node_key);
          try {
            const sub = await api.getSubgraph(s.head_commit, hotspotKeys, 1);
            setGraphNodes(sub.nodes);
            setGraphEdges(sub.edges);
          } catch {
            // Snapshot may not be available
          }

          // Auto-load impact for first hotspot
          const firstTarget = s.hotspots[0].node_key;
          setImpactTarget(firstTarget);
          setImpactLoading(true);
          try {
            const impact = await api.getEgoGraph(s.head_commit, firstTarget, { depth: 2, direction: "both" });
            setImpactNodes(impact.nodes || {});
            setImpactEdges(impact.edges || []);
            setImpactTruncated(impact.truncated || false);
          } catch {
            setImpactNodes({});
            setImpactEdges([]);
          }
          setImpactLoading(false);
        }
      } catch {
        setError("Failed to load score result");
      }
      setLoading(false);
    }
    load();
  }, [params.repoId, params.scoreId]);

  // Collect edge keys from evidence for highlighting
  const { addedEdgeKeys, removedEdgeKeys } = useMemo(() => {
    const added = new Set<string>();
    const removed = new Set<string>();
    if (!score) return { addedEdgeKeys: added, removedEdgeKeys: removed };
    for (const metric of score.breakdown ?? []) {
      for (const ev of metric.evidence ?? []) {
        if (ev.from && ev.to) {
          const key = `${ev.from}->${ev.to}`;
          if (ev.type === "EDGE_ADDED") added.add(key);
          if (ev.type === "EDGE_REMOVED") removed.add(key);
        }
      }
    }
    return { addedEdgeKeys: added, removedEdgeKeys: removed };
  }, [score]);

  // Fetch transitive impact ego graph for a hotspot
  const headCommit = score?.head_commit;
  const fetchImpact = useCallback(async (target: string, depth: number, direction: "deps" | "rdeps" | "both") => {
    if (!headCommit) return;
    setImpactLoading(true);
    setImpactTarget(target);
    setSelectedImpactNode(null);
    try {
      const api = await getAPI();
      const data = await api.getEgoGraph(headCommit, target, { depth, direction });
      setImpactNodes(data.nodes || {});
      setImpactEdges(data.edges || []);
      setImpactTruncated(data.truncated || false);
    } catch {
      setImpactNodes({});
      setImpactEdges([]);
    }
    setImpactLoading(false);
  }, [headCommit]);

  if (loading) {
    return (
      <div className="flex h-full items-center justify-center">
        <div className="text-sm text-zinc-400">Loading score...</div>
      </div>
    );
  }

  if (error || !score) {
    return (
      <div className="flex h-full items-center justify-center">
        <div className="text-sm text-red-400">{error ?? "Score not found"}</div>
      </div>
    );
  }

  const hotspotNodeKeys = new Set((score.hotspots ?? []).map((h) => h.node_key));

  return (
    <div className="p-8">
      <div className="mb-6">
        <Link
          href={`/repos/${params.repoId}`}
          className="mb-2 inline-flex items-center gap-1 text-xs text-zinc-500 hover:text-zinc-700 dark:hover:text-zinc-300"
        >
          <ArrowLeft className="h-3 w-3" />
          Back to repo
        </Link>
        <h1 className="text-2xl font-bold text-zinc-900 dark:text-zinc-100">
          Impact Analysis
        </h1>
        <p className="mt-1 flex items-center gap-2 text-sm text-zinc-500">
          <GitCommit className="h-3.5 w-3.5" />
          {score.base_commit.slice(0, 7)}...{score.head_commit.slice(0, 7)}
          {score.analyzed_at && (
            <span className="text-zinc-400">| {new Date(score.analyzed_at).toLocaleString()}</span>
          )}
        </p>
      </div>

      {/* Grade + Delta Stats */}
      <div className="mb-8 grid grid-cols-5 gap-4">
        <Card>
          <CardContent>
            <div className="flex flex-col items-center gap-2 py-2">
              <GradeBadge grade={score.grade} size="lg" />
              <p className="text-2xl font-bold text-zinc-900 dark:text-zinc-100">
                {score.total_score.toFixed(1)}
              </p>
              <Progress value={Math.min(score.total_score / 30 * 100, 100)} className="w-full" />
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardContent>
            <div className="flex items-center gap-2">
              <Target className="h-4 w-4 text-sky-500" />
              <div>
                <p className="text-xl font-bold text-zinc-900 dark:text-zinc-100">
                  {score.delta_stats.impacted_targets}
                </p>
                <p className="text-[10px] text-zinc-500">Impacted Targets</p>
              </div>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardContent>
            <div className="flex items-center gap-2">
              <Plus className="h-4 w-4 text-emerald-500" />
              <div>
                <p className="text-xl font-bold text-zinc-900 dark:text-zinc-100">
                  {score.delta_stats.added_nodes}
                </p>
                <p className="text-[10px] text-zinc-500">Nodes Added</p>
              </div>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardContent>
            <div className="flex items-center gap-2">
              <Minus className="h-4 w-4 text-red-500" />
              <div>
                <p className="text-xl font-bold text-zinc-900 dark:text-zinc-100">
                  {score.delta_stats.removed_nodes}
                </p>
                <p className="text-[10px] text-zinc-500">Nodes Removed</p>
              </div>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardContent>
            <div className="space-y-1">
              <div className="flex items-center gap-1 text-xs text-zinc-600 dark:text-zinc-400">
                <Plus className="h-3 w-3 text-emerald-500" />
                {score.delta_stats.added_edges} edges
              </div>
              <div className="flex items-center gap-1 text-xs text-zinc-600 dark:text-zinc-400">
                <Minus className="h-3 w-3 text-red-500" />
                {score.delta_stats.removed_edges} edges
              </div>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Score Breakdown + Hotspots */}
      <div className="mb-8 grid grid-cols-2 gap-8">
        <div>
          <ScoreBreakdown metrics={score.breakdown} />
        </div>
        <div>
          <h2 className="mb-3 text-lg font-semibold text-zinc-900 dark:text-zinc-100">
            Structural Hotspots
          </h2>
          <HotspotList hotspots={score.hotspots} />
        </div>
      </div>

      {/* Suggestions */}
      {(score.suggested_actions ?? []).length > 0 && (
        <div className="mb-8">
          <h2 className="mb-3 text-lg font-semibold text-zinc-900 dark:text-zinc-100">
            Suggested Actions
          </h2>
          <Suggestions actions={score.suggested_actions} />
        </div>
      )}

      {/* Direct Change Graph */}
      {Object.keys(graphNodes).length > 0 && (
        <Card className="mb-8">
          <CardHeader>
            <CardTitle>Changed Edges</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="flex">
              <div className={selectedNode ? "flex-1" : "w-full"}>
                <DependencyGraph
                  nodes={graphNodes}
                  edges={graphEdges}
                  highlightedNodes={hotspotNodeKeys}
                  addedEdges={addedEdgeKeys}
                  removedEdges={removedEdgeKeys}
                  onNodeClick={setSelectedNode}
                  width={selectedNode ? 620 : 860}
                  height={400}
                />
              </div>
              {selectedNode && graphNodes[selectedNode] && (
                <div className="w-72">
                  <NodeDetail
                    nodeKey={selectedNode}
                    node={graphNodes[selectedNode]}
                    edges={graphEdges}
                    allNodes={graphNodes}
                    onClose={() => setSelectedNode(null)}
                    onNavigate={setSelectedNode}
                  />
                </div>
              )}
            </div>
          </CardContent>
        </Card>
      )}

      {/* Transitive Impact Graph */}
      {(score.hotspots ?? []).length > 0 && score.head_commit && (
        <Card className="mb-8">
          <CardHeader>
            <div className="flex items-center justify-between">
              <CardTitle className="flex items-center gap-2">
                <Network className="h-5 w-5" />
                Transitive Impact
              </CardTitle>
              <div className="flex items-center gap-3">
                {/* Hotspot target selector */}
                <div className="flex items-center gap-1.5">
                  <span className="text-xs text-zinc-500">Target:</span>
                  <div className="flex gap-1">
                    {(score.hotspots ?? []).map((h) => (
                      <button
                        key={h.node_key}
                        onClick={() => fetchImpact(h.node_key, impactDepth, impactDirection)}
                        className={`rounded px-2 py-1 font-mono text-[10px] transition-colors ${
                          impactTarget === h.node_key
                            ? "bg-amber-100 text-amber-800 dark:bg-amber-900/40 dark:text-amber-300"
                            : "bg-zinc-100 text-zinc-600 hover:bg-zinc-200 dark:bg-zinc-800 dark:text-zinc-400 dark:hover:bg-zinc-700"
                        }`}
                      >
                        {h.node_key.split(":").pop()}
                      </button>
                    ))}
                  </div>
                </div>

                {/* Direction toggle */}
                <div className="flex items-center gap-1 rounded-lg border border-zinc-200 dark:border-zinc-700">
                  {(["deps", "both", "rdeps"] as const).map((dir) => (
                    <button
                      key={dir}
                      onClick={() => {
                        setImpactDirection(dir);
                        if (impactTarget) fetchImpact(impactTarget, impactDepth, dir);
                      }}
                      className={`px-2.5 py-1 text-xs font-medium transition-colors ${
                        impactDirection === dir
                          ? "bg-emerald-100 text-emerald-700 dark:bg-emerald-900 dark:text-emerald-300"
                          : "text-zinc-500 hover:text-zinc-700"
                      }`}
                    >
                      {dir === "deps" ? "New Deps" : dir === "rdeps" ? "Blast Radius" : "Both"}
                    </button>
                  ))}
                </div>

                {/* Depth slider */}
                <div className="flex items-center gap-2">
                  <label className="text-xs text-zinc-500">Depth:</label>
                  <input
                    type="range"
                    min={1}
                    max={5}
                    value={impactDepth}
                    onChange={(e) => {
                      const newDepth = Number(e.target.value);
                      setImpactDepth(newDepth);
                      if (impactTarget) fetchImpact(impactTarget, newDepth, impactDirection);
                    }}
                    className="w-20"
                  />
                  <span className="text-xs font-medium text-zinc-700 dark:text-zinc-300">{impactDepth}</span>
                </div>
              </div>
            </div>

            {impactTarget && (
              <p className="mt-2 text-xs text-zinc-500">
                {impactDirection === "deps" && "Showing what this target transitively depends on — new dependencies pulled into the graph."}
                {impactDirection === "rdeps" && "Showing what transitively depends on this target — everything affected by this change."}
                {impactDirection === "both" && "Showing the full transitive neighborhood — new dependencies and blast radius."}
                {impactTruncated && " (capped at 500 nodes)"}
              </p>
            )}
          </CardHeader>
          <CardContent>
            {impactLoading ? (
              <div className="flex h-[500px] items-center justify-center">
                <div className="text-sm text-zinc-400">Loading transitive graph...</div>
              </div>
            ) : Object.keys(impactNodes).length === 0 ? (
              <div className="flex h-[500px] items-center justify-center">
                <p className="text-sm text-zinc-400">Select a hotspot target to view its transitive impact</p>
              </div>
            ) : (
              <div className="flex">
                <div className={selectedImpactNode ? "flex-1" : "w-full"}>
                  <DependencyGraph
                    nodes={impactNodes}
                    edges={impactEdges}
                    highlightedNodes={hotspotNodeKeys}
                    addedEdges={addedEdgeKeys}
                    removedEdges={removedEdgeKeys}
                    onNodeClick={setSelectedImpactNode}
                    width={selectedImpactNode ? 620 : 860}
                    height={500}
                  />
                </div>
                {selectedImpactNode && impactNodes[selectedImpactNode] && (
                  <div className="w-72">
                    <NodeDetail
                      nodeKey={selectedImpactNode}
                      node={impactNodes[selectedImpactNode]}
                      edges={impactEdges}
                      allNodes={impactNodes}
                      onClose={() => setSelectedImpactNode(null)}
                      onNavigate={setSelectedImpactNode}
                    />
                  </div>
                )}
              </div>
            )}

            {Object.keys(impactNodes).length > 0 && (
              <div className="mt-2 text-xs text-zinc-400">
                {Object.keys(impactNodes).length} nodes | {impactEdges.length} edges
              </div>
            )}
          </CardContent>
        </Card>
      )}
    </div>
  );
}
