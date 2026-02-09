"use client";

import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import { ArrowLeft, Plus, Minus, Target, GitCommit } from "lucide-react";
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

export default function PRImpactPage() {
  const params = useParams<{ repoId: string; prNumber: string }>();
  const [score, setScore] = useState<ScoreResult | null>(null);
  const [graphNodes, setGraphNodes] = useState<Record<string, Node>>({});
  const [graphEdges, setGraphEdges] = useState<Edge[]>([]);
  const [selectedNode, setSelectedNode] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    async function load() {
      const api = await getAPI();
      const s = await api.getPRImpact(params.repoId, Number(params.prNumber));
      setScore(s);

      // Load graph data
      const snapshot = await api.getSnapshot("snap-001");
      setGraphNodes(snapshot.nodes);
      setGraphEdges(snapshot.edges);
      setLoading(false);
    }
    load();
  }, [params.repoId, params.prNumber]);

  if (loading || !score) {
    return (
      <div className="flex h-full items-center justify-center">
        <div className="text-sm text-zinc-400">Loading...</div>
      </div>
    );
  }

  const hotspotNodeKeys = new Set(score.hotspots.map((h) => h.node_key));

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
          PR #{params.prNumber} Impact Analysis
        </h1>
        <p className="mt-1 flex items-center gap-2 text-sm text-zinc-500">
          <GitCommit className="h-3.5 w-3.5" />
          {score.base_commit.slice(0, 7)}...{score.head_commit.slice(0, 7)}
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
      {score.suggested_actions.length > 0 && (
        <div className="mb-8">
          <h2 className="mb-3 text-lg font-semibold text-zinc-900 dark:text-zinc-100">
            Suggested Actions
          </h2>
          <Suggestions actions={score.suggested_actions} />
        </div>
      )}

      {/* Dependency Graph */}
      <Card className="mb-8">
        <CardHeader>
          <CardTitle>Dependency Graph (Delta Subgraph)</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex">
            <div className={selectedNode ? "flex-1" : "w-full"}>
              <DependencyGraph
                nodes={graphNodes}
                edges={graphEdges}
                highlightedNodes={hotspotNodeKeys}
                onNodeClick={setSelectedNode}
                width={selectedNode ? 620 : 860}
                height={500}
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
    </div>
  );
}
