"use client";

import { useEffect, useState, useMemo, useCallback } from "react";
import { useParams } from "next/navigation";
import { Search, ChevronRight, Map, Crosshair, Flame, Route } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { DependencyGraph } from "@/components/graph/dependency-graph";
import { PackageGraph } from "@/components/graph/package-graph";
import { PathGraph } from "@/components/graph/path-graph";
import { NodeDetail } from "@/components/graph/node-detail";
import type { Node, Edge, PackageNode, PackageEdge, PathResult } from "@/lib/types";

type TabId = "packages" | "explorer" | "hotspots" | "pathfinder";

interface SnapshotInfo {
  id: string;
  commit_sha: string;
  node_count: number;
  edge_count: number;
  package_count: number;
}

const BASE_URL = process.env.NEXT_PUBLIC_API_BASE_URL || "http://localhost:7700";

async function fetchJSON<T>(path: string): Promise<T> {
  const res = await fetch(`${BASE_URL}${path}`, {
    headers: { "Content-Type": "application/json" },
  });
  if (!res.ok) throw new Error(`API error: ${res.status}`);
  return res.json() as Promise<T>;
}

export default function GraphExplorerPage() {
  const params = useParams<{ repoId: string }>();
  const [tab, setTab] = useState<TabId>("packages");
  const [snapshotId, setSnapshotId] = useState<string | null>(null);
  const [snapInfo, setSnapInfo] = useState<SnapshotInfo | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Package Map state
  const [pkgNodes, setPkgNodes] = useState<Record<string, PackageNode>>({});
  const [pkgEdges, setPkgEdges] = useState<PackageEdge[]>([]);
  const [pkgTruncated, setPkgTruncated] = useState(false);
  const [hideTests, setHideTests] = useState(true);
  const [hideExternal, setHideExternal] = useState(true);
  const [minEdgeWeight, setMinEdgeWeight] = useState(1);
  const [drillPath, setDrillPath] = useState<string[]>([]);
  const [drillNodes, setDrillNodes] = useState<Record<string, Node>>({});
  const [drillEdges, setDrillEdges] = useState<Edge[]>([]);
  const [selectedDrillNode, setSelectedDrillNode] = useState<string | null>(null);

  // Target Explorer state
  const [egoSearch, setEgoSearch] = useState("");
  const [egoTarget, setEgoTarget] = useState<string | null>(null);
  const [egoNodes, setEgoNodes] = useState<Record<string, Node>>({});
  const [egoEdges, setEgoEdges] = useState<Edge[]>([]);
  const [egoTruncated, setEgoTruncated] = useState(false);
  const [egoDepth, setEgoDepth] = useState(2);
  const [egoDirection, setEgoDirection] = useState<"deps" | "rdeps" | "both">("both");
  const [selectedEgoNode, setSelectedEgoNode] = useState<string | null>(null);
  const [egoLoading, setEgoLoading] = useState(false);

  // Hotspots (derived from package data)

  // Path Finder state
  const [pathFromSearch, setPathFromSearch] = useState("");
  const [pathToSearch, setPathToSearch] = useState("");
  const [pathFrom, setPathFrom] = useState<string | null>(null);
  const [, setPathTo] = useState<string | null>(null);
  const [pathResult, setPathResult] = useState<PathResult | null>(null);
  const [pathLoading, setPathLoading] = useState(false);
  const [selectedPathNode, setSelectedPathNode] = useState<string | null>(null);
  const [highlightedPathIndex, setHighlightedPathIndex] = useState<number | null>(null);

  // Load snapshot list on mount
  useEffect(() => {
    async function load() {
      try {
        const snapList = await fetchJSON<SnapshotInfo[]>("/api/snapshots/");
        if (snapList.length > 0) {
          setSnapInfo(snapList[0]);
          setSnapshotId(snapList[0].commit_sha);
        } else {
          setError("No snapshots available");
        }
      } catch {
        setError("Could not connect to API server");
      }
      setLoading(false);
    }
    load();
  }, [params.repoId]);

  // Load package graph when snapshot is ready or filters change
  const [pkgLoading, setPkgLoading] = useState(false);

  useEffect(() => {
    if (!snapshotId) return;
    async function loadPackages() {
      setPkgLoading(true);
      try {
        const qs = new URLSearchParams();
        if (hideTests) qs.set("hide_tests", "true");
        if (hideExternal) qs.set("hide_external", "true");
        if (minEdgeWeight > 1) qs.set("min_edge_weight", String(minEdgeWeight));
        const qsStr = qs.toString();
        const data = await fetchJSON<{ nodes: Record<string, PackageNode>; edges: PackageEdge[]; truncated: boolean }>(
          `/api/snapshots/${snapshotId}/packages${qsStr ? `?${qsStr}` : ""}`
        );

        let nodes = data.nodes || {};
        let edges: PackageEdge[] = data.edges || [];

        // Defensive client-side cap: if backend returns too many nodes
        // (e.g. old server returning raw targets instead of packages), cap here.
        const MAX_RENDERABLE = 500;
        const nodeKeys = Object.keys(nodes);
        if (nodeKeys.length > MAX_RENDERABLE) {
          // Compute degree for ranking
          const deg: Record<string, number> = {};
          for (const e of edges) {
            deg[e.from] = (deg[e.from] || 0) + 1;
            deg[e.to] = (deg[e.to] || 0) + 1;
          }
          // Keep top N by degree
          const sorted = nodeKeys.sort((a, b) => (deg[b] || 0) - (deg[a] || 0));
          const keep = new Set(sorted.slice(0, MAX_RENDERABLE));
          const capped: Record<string, PackageNode> = {};
          for (const k of keep) capped[k] = nodes[k];
          nodes = capped;
          edges = edges.filter((e) => keep.has(e.from) && keep.has(e.to));
        }

        setPkgNodes(nodes);
        setPkgEdges(edges);
        setPkgTruncated(data.truncated || nodeKeys.length > MAX_RENDERABLE);
      } catch {
        // API not available for packages
      }
      setPkgLoading(false);
    }
    loadPackages();
  }, [snapshotId, hideTests, hideExternal, minEdgeWeight]);

  // Compute hotspots from package data
  const hotspots = useMemo(() => {
    const degreeMap: Record<string, { total: number; inDeg: number }> = {};
    for (const e of pkgEdges) {
      const w = e.weight ?? 1;
      if (!degreeMap[e.from]) degreeMap[e.from] = { total: 0, inDeg: 0 };
      degreeMap[e.from].total += w;

      if (!degreeMap[e.to]) degreeMap[e.to] = { total: 0, inDeg: 0 };
      degreeMap[e.to].total += w;
      degreeMap[e.to].inDeg += w;
    }

    return Object.entries(pkgNodes)
      .map(([pkg, node]) => {
        const deg = degreeMap[pkg];
        return {
          pkg,
          degree: deg?.total || 0,
          inDegree: deg?.inDeg || 0,
          targetCount: node.target_count ?? 0,
        };
      })
      .sort((a, b) => b.inDegree - a.inDegree);
  }, [pkgNodes, pkgEdges]);

  // Drill into a package (fetch its targets)
  const drillIntoPackage = useCallback(async (pkg: string) => {
    if (!snapshotId) return;
    try {
      const data = await fetchJSON<{ nodes: Record<string, Node>; edges: Edge[] }>(
        `/api/snapshots/${snapshotId}/subgraph?root=${encodeURIComponent(pkg)}&depth=1`
      );
      setDrillNodes(data.nodes || {});
      setDrillEdges(data.edges || []);
      setDrillPath((prev) => [...prev, pkg]);
      setSelectedDrillNode(null);
    } catch {
      // Failed to drill
    }
  }, [snapshotId]);

  // Fetch ego graph
  const fetchEgoGraph = useCallback(async (target: string, depth: number, direction: "deps" | "rdeps" | "both") => {
    if (!snapshotId) return;
    setEgoLoading(true);
    setEgoTarget(target);
    setSelectedEgoNode(null);
    try {
      const params = new URLSearchParams({
        target,
        depth: String(depth),
        direction,
      });
      const data = await fetchJSON<{ nodes: Record<string, Node>; edges: Edge[]; truncated: boolean }>(
        `/api/snapshots/${snapshotId}/ego?${params}`
      );
      setEgoNodes(data.nodes || {});
      setEgoEdges(data.edges || []);
      setEgoTruncated(data.truncated || false);
    } catch {
      setEgoNodes({});
      setEgoEdges([]);
    }
    setEgoLoading(false);
  }, [snapshotId]);

  // All target keys for search autocomplete (from package nodes -> build label patterns)
  const egoSearchResults = useMemo(() => {
    if (!egoSearch || egoSearch.length < 2) return [];
    const q = egoSearch.toLowerCase();
    // Search through package names as likely targets
    return Object.keys(pkgNodes)
      .filter((k) => k.toLowerCase().includes(q))
      .slice(0, 10);
  }, [pkgNodes, egoSearch]);

  // Path Finder search autocomplete
  const pathFromResults = useMemo(() => {
    if (!pathFromSearch || pathFromSearch.length < 2) return [];
    const q = pathFromSearch.toLowerCase();
    return Object.keys(pkgNodes)
      .filter((k) => k.toLowerCase().includes(q))
      .slice(0, 10);
  }, [pkgNodes, pathFromSearch]);

  const pathToResults = useMemo(() => {
    if (!pathToSearch || pathToSearch.length < 2) return [];
    const q = pathToSearch.toLowerCase();
    return Object.keys(pkgNodes)
      .filter((k) => k.toLowerCase().includes(q))
      .slice(0, 10);
  }, [pkgNodes, pathToSearch]);

  // Fetch path
  const fetchPath = useCallback(async (from: string, to: string) => {
    if (!snapshotId) return;
    setPathLoading(true);
    setPathFrom(from);
    setPathTo(to);
    setSelectedPathNode(null);
    setHighlightedPathIndex(null);
    try {
      const params = new URLSearchParams({ from, to, max_paths: "10" });
      const data = await fetchJSON<PathResult>(
        `/api/snapshots/${snapshotId}/path?${params}`
      );
      setPathResult(data);
    } catch {
      setPathResult(null);
    }
    setPathLoading(false);
  }, [snapshotId]);

  if (loading) {
    return (
      <div className="flex h-full items-center justify-center">
        <div className="text-sm text-zinc-400">Loading snapshots...</div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex h-full items-center justify-center">
        <div className="text-sm text-red-400">{error}</div>
      </div>
    );
  }

  const tabs: { id: TabId; label: string; icon: React.ReactNode }[] = [
    { id: "packages", label: "Package Map", icon: <Map className="h-4 w-4" /> },
    { id: "explorer", label: "Target Explorer", icon: <Crosshair className="h-4 w-4" /> },
    { id: "hotspots", label: "Hotspots", icon: <Flame className="h-4 w-4" /> },
    { id: "pathfinder", label: "Path Finder", icon: <Route className="h-4 w-4" /> },
  ];

  return (
    <div className="flex h-full flex-col p-8">
      <div className="mb-4">
        <h1 className="text-2xl font-bold text-zinc-900 dark:text-zinc-100">Graph Explorer</h1>
        {snapInfo && (
          <p className="mt-1 text-sm text-zinc-500">
            Snapshot {snapInfo.commit_sha.slice(0, 8)} &mdash; {snapInfo.node_count.toLocaleString()} targets, {snapInfo.package_count.toLocaleString()} packages
          </p>
        )}
      </div>

      {/* Tabs */}
      <div className="mb-4 flex border-b border-zinc-200 dark:border-zinc-800">
        {tabs.map((t) => (
          <button
            key={t.id}
            onClick={() => setTab(t.id)}
            className={`flex items-center gap-2 border-b-2 px-4 py-2 text-sm font-medium transition-colors ${
              tab === t.id
                ? "border-emerald-500 text-emerald-600 dark:text-emerald-400"
                : "border-transparent text-zinc-500 hover:text-zinc-700 dark:hover:text-zinc-300"
            }`}
          >
            {t.icon}
            {t.label}
          </button>
        ))}
      </div>

      {/* Tab 1: Package Map */}
      {tab === "packages" && (
        <div className="flex min-h-0 flex-1 flex-col">
          {/* Controls */}
          <div className="mb-3 flex flex-wrap items-center gap-3">
            <label className="flex items-center gap-1.5 text-xs text-zinc-500">
              <input
                type="checkbox"
                checked={hideTests}
                onChange={(e) => setHideTests(e.target.checked)}
                className="rounded"
              />
              Hide tests
            </label>
            <label className="flex items-center gap-1.5 text-xs text-zinc-500">
              <input
                type="checkbox"
                checked={hideExternal}
                onChange={(e) => setHideExternal(e.target.checked)}
                className="rounded"
              />
              Hide external
            </label>
            <div className="flex items-center gap-2">
              <label className="text-xs text-zinc-500">Min edge weight:</label>
              <input
                type="range"
                min={1}
                max={20}
                value={minEdgeWeight}
                onChange={(e) => setMinEdgeWeight(Number(e.target.value))}
                className="w-24"
              />
              <span className="text-xs font-medium text-zinc-700 dark:text-zinc-300">{minEdgeWeight}</span>
            </div>
            <span className="ml-auto text-xs text-zinc-400">
              {Object.keys(pkgNodes).length} packages | {pkgEdges.length} edges
              {pkgTruncated && " (truncated to top 500)"}
            </span>
          </div>

          {/* Breadcrumbs */}
          {drillPath.length > 0 && (
            <div className="mb-3 flex items-center gap-1 text-sm">
              <button
                onClick={() => {
                  setDrillPath([]);
                  setDrillNodes({});
                  setDrillEdges([]);
                  setSelectedDrillNode(null);
                }}
                className="text-emerald-600 hover:underline dark:text-emerald-400"
              >
                All Packages
              </button>
              {drillPath.map((pkg, i) => (
                <span key={pkg} className="flex items-center gap-1">
                  <ChevronRight className="h-3 w-3 text-zinc-400" />
                  <button
                    onClick={() => {
                      setDrillPath(drillPath.slice(0, i + 1));
                      if (i < drillPath.length - 1) {
                        drillIntoPackage(pkg);
                      }
                    }}
                    className={
                      i === drillPath.length - 1
                        ? "font-medium text-zinc-900 dark:text-zinc-100"
                        : "text-emerald-600 hover:underline dark:text-emerald-400"
                    }
                  >
                    {pkg.replace(/^\/\//, "")}
                  </button>
                </span>
              ))}
            </div>
          )}

          {/* Graph */}
          <Card className="min-h-0 flex-1">
            <CardContent className="h-full p-0">
              <div className="flex h-[calc(100vh-16rem)]">
                {pkgLoading ? (
                  <div className="flex w-full items-center justify-center">
                    <div className="text-sm text-zinc-400">Loading package graph...</div>
                  </div>
                ) : Object.keys(pkgNodes).length === 0 ? (
                  <div className="flex w-full items-center justify-center">
                    <div className="text-center">
                      <Map className="mx-auto mb-3 h-10 w-10 text-zinc-300 dark:text-zinc-600" />
                      <p className="text-sm text-zinc-500">No package data available. Make sure the API server is rebuilt and running.</p>
                    </div>
                  </div>
                ) : drillPath.length === 0 ? (
                  <div className="w-full">
                    <PackageGraph
                      nodes={pkgNodes}
                      edges={pkgEdges}
                      onPackageClick={drillIntoPackage}
                      minEdgeWeight={minEdgeWeight}
                      width={1200}
                      height={Math.max(600, 700)}
                    />
                  </div>
                ) : (
                  <>
                    <div className={selectedDrillNode ? "flex-1" : "w-full"}>
                      <DependencyGraph
                        nodes={drillNodes}
                        edges={drillEdges}
                        highlightedNodes={selectedDrillNode ? new Set([selectedDrillNode]) : undefined}
                        onNodeClick={setSelectedDrillNode}
                        width={selectedDrillNode ? 900 : 1200}
                        height={Math.max(600, 700)}
                      />
                    </div>
                    {selectedDrillNode && drillNodes[selectedDrillNode] && (
                      <div className="w-80 border-l border-zinc-200 dark:border-zinc-800">
                        <NodeDetail
                          nodeKey={selectedDrillNode}
                          node={drillNodes[selectedDrillNode]}
                          edges={drillEdges}
                          allNodes={drillNodes}
                          onClose={() => setSelectedDrillNode(null)}
                          onNavigate={setSelectedDrillNode}
                        />
                      </div>
                    )}
                  </>
                )}
              </div>
            </CardContent>
          </Card>
        </div>
      )}

      {/* Tab 2: Target Explorer */}
      {tab === "explorer" && (
        <div className="flex min-h-0 flex-1 flex-col">
          {/* Search bar */}
          <div className="mb-3 flex flex-wrap items-center gap-3">
            <div className="relative flex-1 max-w-md">
              <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-zinc-400" />
              <input
                type="text"
                placeholder="Search target or package (e.g. //go/auth)..."
                value={egoSearch}
                onChange={(e) => setEgoSearch(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter" && egoSearch) {
                    fetchEgoGraph(egoSearch, egoDepth, egoDirection);
                    setEgoSearch("");
                  }
                }}
                className="h-9 w-full rounded-lg border border-zinc-200 bg-white pl-9 pr-3 text-sm outline-none focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500 dark:border-zinc-700 dark:bg-zinc-900"
              />
              {egoSearchResults.length > 0 && egoSearch && (
                <div className="absolute left-0 top-full z-20 mt-1 max-h-48 w-full overflow-y-auto rounded-lg border border-zinc-200 bg-white shadow-lg dark:border-zinc-700 dark:bg-zinc-900">
                  {egoSearchResults.map((key) => (
                    <button
                      key={key}
                      onClick={() => {
                        fetchEgoGraph(key, egoDepth, egoDirection);
                        setEgoSearch("");
                      }}
                      className="block w-full px-3 py-2 text-left font-mono text-xs hover:bg-zinc-50 dark:hover:bg-zinc-800"
                    >
                      {key}
                    </button>
                  ))}
                </div>
              )}
            </div>

            <div className="flex items-center gap-2">
              <label className="text-xs text-zinc-500">Depth:</label>
              <input
                type="range"
                min={1}
                max={5}
                value={egoDepth}
                onChange={(e) => {
                  const newDepth = Number(e.target.value);
                  setEgoDepth(newDepth);
                  if (egoTarget) fetchEgoGraph(egoTarget, newDepth, egoDirection);
                }}
                className="w-20"
              />
              <span className="text-xs font-medium text-zinc-700 dark:text-zinc-300">{egoDepth}</span>
            </div>

            <div className="flex items-center gap-1 rounded-lg border border-zinc-200 dark:border-zinc-700">
              {(["deps", "both", "rdeps"] as const).map((dir) => (
                <button
                  key={dir}
                  onClick={() => {
                    setEgoDirection(dir);
                    if (egoTarget) fetchEgoGraph(egoTarget, egoDepth, dir);
                  }}
                  className={`px-2.5 py-1 text-xs font-medium transition-colors ${
                    egoDirection === dir
                      ? "bg-emerald-100 text-emerald-700 dark:bg-emerald-900 dark:text-emerald-300"
                      : "text-zinc-500 hover:text-zinc-700"
                  }`}
                >
                  {dir === "deps" ? "Deps" : dir === "rdeps" ? "Reverse" : "Both"}
                </button>
              ))}
            </div>

            {egoTarget && (
              <span className="ml-auto text-xs text-zinc-400">
                {Object.keys(egoNodes).length} nodes | {egoEdges.length} edges
                {egoTruncated && " (capped at 500)"}
              </span>
            )}
          </div>

          {/* Graph */}
          <Card className="min-h-0 flex-1">
            <CardContent className="h-full p-0">
              <div className="flex h-[calc(100vh-16rem)]">
                {egoLoading ? (
                  <div className="flex w-full items-center justify-center">
                    <div className="text-sm text-zinc-400">Loading ego graph...</div>
                  </div>
                ) : !egoTarget ? (
                  <div className="flex w-full items-center justify-center">
                    <div className="text-center">
                      <Crosshair className="mx-auto mb-3 h-10 w-10 text-zinc-300 dark:text-zinc-600" />
                      <p className="text-sm text-zinc-500">Search for a target or package to explore its dependency neighborhood</p>
                    </div>
                  </div>
                ) : Object.keys(egoNodes).length === 0 ? (
                  <div className="flex w-full items-center justify-center">
                    <p className="text-sm text-zinc-400">No matching targets found for &quot;{egoTarget}&quot;</p>
                  </div>
                ) : (
                  <>
                    <div className={selectedEgoNode ? "flex-1" : "w-full"}>
                      <DependencyGraph
                        nodes={egoNodes}
                        edges={egoEdges}
                        highlightedNodes={selectedEgoNode ? new Set([selectedEgoNode]) : undefined}
                        onNodeClick={setSelectedEgoNode}
                        width={selectedEgoNode ? 900 : 1200}
                        height={Math.max(600, 700)}
                      />
                    </div>
                    {selectedEgoNode && egoNodes[selectedEgoNode] && (
                      <div className="w-80 border-l border-zinc-200 dark:border-zinc-800">
                        <NodeDetail
                          nodeKey={selectedEgoNode}
                          node={egoNodes[selectedEgoNode]}
                          edges={egoEdges}
                          allNodes={egoNodes}
                          onClose={() => setSelectedEgoNode(null)}
                          onNavigate={setSelectedEgoNode}
                        />
                      </div>
                    )}
                  </>
                )}
              </div>
            </CardContent>
          </Card>
        </div>
      )}

      {/* Tab 3: Hotspots */}
      {tab === "hotspots" && (
        <div className="flex min-h-0 flex-1 flex-col">
          <p className="mb-3 text-sm text-zinc-500">
            Packages ranked by in-degree (most depended upon). Click to explore in Target Explorer.
          </p>
          <Card className="min-h-0 flex-1 overflow-hidden">
            <CardContent className="h-full overflow-y-auto p-0">
              <table className="w-full text-sm">
                <thead className="sticky top-0 bg-white dark:bg-zinc-950">
                  <tr className="border-b border-zinc-200 text-left text-xs font-medium uppercase tracking-wider text-zinc-400 dark:border-zinc-800">
                    <th className="px-4 py-2 w-10">#</th>
                    <th className="px-4 py-2">Package</th>
                    <th className="px-4 py-2 w-24 text-right">In-Degree</th>
                    <th className="px-4 py-2 w-24 text-right">Degree</th>
                    <th className="px-4 py-2 w-24 text-right">Targets</th>
                    <th className="px-4 py-2 w-48">In-Degree Distribution</th>
                  </tr>
                </thead>
                <tbody>
                  {hotspots.slice(0, 100).map((h, i) => {
                    const maxInDegree = hotspots[0]?.inDegree || 1;
                    const barWidth = Math.max(2, (h.inDegree / maxInDegree) * 100);
                    return (
                      <tr
                        key={h.pkg}
                        onClick={() => {
                          setTab("explorer");
                          setEgoSearch("");
                          fetchEgoGraph(h.pkg, egoDepth, egoDirection);
                        }}
                        className="cursor-pointer border-b border-zinc-100 hover:bg-zinc-50 dark:border-zinc-900 dark:hover:bg-zinc-900"
                      >
                        <td className="px-4 py-2 text-zinc-400">{i + 1}</td>
                        <td className="px-4 py-2 font-mono text-xs text-zinc-900 dark:text-zinc-100">
                          {h.pkg}
                        </td>
                        <td className="px-4 py-2 text-right font-medium text-zinc-700 dark:text-zinc-300">
                          {h.inDegree}
                        </td>
                        <td className="px-4 py-2 text-right text-zinc-500">{h.degree}</td>
                        <td className="px-4 py-2 text-right text-zinc-500">{h.targetCount}</td>
                        <td className="px-4 py-2">
                          <div className="h-3 w-full rounded-full bg-zinc-100 dark:bg-zinc-800">
                            <div
                              className="h-3 rounded-full bg-emerald-500"
                              style={{ width: `${barWidth}%` }}
                            />
                          </div>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
              {hotspots.length === 0 && (
                <div className="flex h-40 items-center justify-center">
                  <p className="text-sm text-zinc-400">No package data loaded</p>
                </div>
              )}
            </CardContent>
          </Card>
        </div>
      )}

      {/* Tab 4: Path Finder */}
      {tab === "pathfinder" && (
        <div className="flex min-h-0 flex-1 flex-col">
          {/* Search inputs */}
          <div className="mb-3 flex flex-wrap items-center gap-3">
            <div className="relative flex-1 max-w-xs">
              <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-zinc-400" />
              <input
                type="text"
                placeholder="From (e.g. //go/auth)..."
                value={pathFromSearch}
                onChange={(e) => setPathFromSearch(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter" && pathFromSearch && pathToSearch) {
                    fetchPath(pathFromSearch, pathToSearch);
                    setPathFromSearch("");
                    setPathToSearch("");
                  }
                }}
                className="h-9 w-full rounded-lg border border-zinc-200 bg-white pl-9 pr-3 text-sm outline-none focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500 dark:border-zinc-700 dark:bg-zinc-900"
              />
              {pathFromResults.length > 0 && pathFromSearch && (
                <div className="absolute left-0 top-full z-20 mt-1 max-h-48 w-full overflow-y-auto rounded-lg border border-zinc-200 bg-white shadow-lg dark:border-zinc-700 dark:bg-zinc-900">
                  {pathFromResults.map((key) => (
                    <button
                      key={key}
                      onClick={() => {
                        setPathFromSearch(key);
                      }}
                      className="block w-full px-3 py-2 text-left font-mono text-xs hover:bg-zinc-50 dark:hover:bg-zinc-800"
                    >
                      {key}
                    </button>
                  ))}
                </div>
              )}
            </div>

            <span className="text-zinc-400 text-sm">&rarr;</span>

            <div className="relative flex-1 max-w-xs">
              <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-zinc-400" />
              <input
                type="text"
                placeholder="To (e.g. //go/util)..."
                value={pathToSearch}
                onChange={(e) => setPathToSearch(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter" && pathFromSearch && pathToSearch) {
                    fetchPath(pathFromSearch, pathToSearch);
                    setPathFromSearch("");
                    setPathToSearch("");
                  }
                }}
                className="h-9 w-full rounded-lg border border-zinc-200 bg-white pl-9 pr-3 text-sm outline-none focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500 dark:border-zinc-700 dark:bg-zinc-900"
              />
              {pathToResults.length > 0 && pathToSearch && (
                <div className="absolute left-0 top-full z-20 mt-1 max-h-48 w-full overflow-y-auto rounded-lg border border-zinc-200 bg-white shadow-lg dark:border-zinc-700 dark:bg-zinc-900">
                  {pathToResults.map((key) => (
                    <button
                      key={key}
                      onClick={() => {
                        setPathToSearch(key);
                      }}
                      className="block w-full px-3 py-2 text-left font-mono text-xs hover:bg-zinc-50 dark:hover:bg-zinc-800"
                    >
                      {key}
                    </button>
                  ))}
                </div>
              )}
            </div>

            <button
              onClick={() => {
                if (pathFromSearch && pathToSearch) {
                  fetchPath(pathFromSearch, pathToSearch);
                  setPathFromSearch("");
                  setPathToSearch("");
                }
              }}
              disabled={!pathFromSearch || !pathToSearch}
              className="h-9 rounded-lg bg-emerald-600 px-4 text-sm font-medium text-white hover:bg-emerald-700 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              Find Path
            </button>

            {pathResult && pathResult.paths.length > 0 && (
              <span className="ml-auto text-xs text-zinc-400">
                {pathResult.paths.length} path{pathResult.paths.length !== 1 ? "s" : ""} | {pathResult.path_length} hop{pathResult.path_length !== 1 ? "s" : ""} | {Object.keys(pathResult.nodes).length} nodes
              </span>
            )}
          </div>

          {/* Graph + paths list */}
          <Card className="min-h-0 flex-1">
            <CardContent className="h-full p-0">
              <div className="flex h-[calc(100vh-16rem)]">
                {pathLoading ? (
                  <div className="flex w-full items-center justify-center">
                    <div className="text-sm text-zinc-400">Finding shortest paths...</div>
                  </div>
                ) : !pathFrom ? (
                  <div className="flex w-full items-center justify-center">
                    <div className="text-center">
                      <Route className="mx-auto mb-3 h-10 w-10 text-zinc-300 dark:text-zinc-600" />
                      <p className="text-sm text-zinc-500">Enter two targets to find the shortest dependency path between them</p>
                    </div>
                  </div>
                ) : pathResult && pathResult.paths.length === 0 ? (
                  <div className="flex w-full items-center justify-center">
                    <div className="text-center">
                      <Route className="mx-auto mb-3 h-10 w-10 text-zinc-300 dark:text-zinc-600" />
                      <p className="text-sm text-zinc-500">No path found from &quot;{pathResult.from}&quot; to &quot;{pathResult.to}&quot;</p>
                    </div>
                  </div>
                ) : pathResult ? (
                  <div className="flex w-full flex-col">
                    <div className={`flex min-h-0 flex-1 ${selectedPathNode ? "" : ""}`}>
                      <div className={selectedPathNode ? "flex-1" : "w-full"}>
                        <PathGraph
                          nodes={pathResult.nodes}
                          edges={pathResult.edges}
                          paths={pathResult.paths}
                          highlightedPath={highlightedPathIndex}
                          onNodeClick={setSelectedPathNode}
                          width={selectedPathNode ? 900 : 1200}
                          height={Math.max(400, 500)}
                        />
                      </div>
                      {selectedPathNode && pathResult.nodes[selectedPathNode] && (
                        <div className="w-80 border-l border-zinc-200 dark:border-zinc-800">
                          <NodeDetail
                            nodeKey={selectedPathNode}
                            node={pathResult.nodes[selectedPathNode]}
                            edges={pathResult.edges}
                            allNodes={pathResult.nodes}
                            onClose={() => setSelectedPathNode(null)}
                            onNavigate={setSelectedPathNode}
                          />
                        </div>
                      )}
                    </div>

                    {/* Path list breadcrumbs */}
                    {pathResult.paths.length > 1 && (
                      <div className="border-t border-zinc-200 px-4 py-3 dark:border-zinc-800">
                        <div className="mb-2 flex items-center justify-between">
                          <span className="text-xs font-medium uppercase tracking-wider text-zinc-400">
                            Shortest Paths ({pathResult.paths.length})
                          </span>
                          {highlightedPathIndex !== null && (
                            <button
                              onClick={() => setHighlightedPathIndex(null)}
                              className="text-xs text-emerald-600 hover:underline dark:text-emerald-400"
                            >
                              Show all
                            </button>
                          )}
                        </div>
                        <div className="flex flex-col gap-1.5 max-h-32 overflow-y-auto">
                          {pathResult.paths.map((path, idx) => (
                            <button
                              key={idx}
                              onClick={() => setHighlightedPathIndex(highlightedPathIndex === idx ? null : idx)}
                              className={`flex items-center gap-1 rounded px-2 py-1 text-left font-mono text-xs transition-colors ${
                                highlightedPathIndex === idx
                                  ? "bg-emerald-50 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300"
                                  : "text-zinc-600 hover:bg-zinc-50 dark:text-zinc-400 dark:hover:bg-zinc-900"
                              }`}
                            >
                              {path.map((node, ni) => (
                                <span key={ni} className="flex items-center gap-1">
                                  {ni > 0 && <ChevronRight className="h-3 w-3 flex-shrink-0 text-zinc-300" />}
                                  <span className="truncate max-w-[120px]">{node.split(":").pop()}</span>
                                </span>
                              ))}
                            </button>
                          ))}
                        </div>
                      </div>
                    )}
                  </div>
                ) : null}
              </div>
            </CardContent>
          </Card>
        </div>
      )}
    </div>
  );
}
