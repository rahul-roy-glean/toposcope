"use client";

import { useEffect, useRef, useState, useCallback } from "react";
import * as d3 from "d3";
import type { Node, Edge } from "@/lib/types";

interface PathGraphProps {
  nodes: Record<string, Node>;
  edges: Edge[];
  paths: string[][];
  highlightedPath?: number | null;
  onNodeClick?: (nodeKey: string) => void;
  width?: number;
  height?: number;
}

function hashColor(str: string): string {
  let hash = 0;
  for (let i = 0; i < str.length; i++) {
    hash = str.charCodeAt(i) + ((hash << 5) - hash);
  }
  const hue = ((hash % 360) + 360) % 360;
  return `hsl(${hue}, 55%, 50%)`;
}

function safeNode(n: Node): Node {
  return {
    ...n,
    tags: n.tags ?? [],
    visibility: n.visibility ?? [],
    package: n.package ?? "",
    kind: n.kind ?? "",
  };
}

export function PathGraph({
  nodes,
  edges,
  paths,
  highlightedPath,
  onNodeClick,
  width = 1200,
  height = 600,
}: PathGraphProps) {
  const svgRef = useRef<SVGSVGElement>(null);
  const [hoveredNode, setHoveredNode] = useState<string | null>(null);

  const renderGraph = useCallback(() => {
    if (!svgRef.current || paths.length === 0) return;

    const isDark = document.documentElement.classList.contains("dark");
    const contextEdgeColor = isDark ? "#52525b" : "#d4d4d8";
    const contextEdgeOpacity = isDark ? 0.5 : 0.3;
    const labelDimColor = isDark ? "#71717a" : "#a1a1aa";
    const labelBrightColor = isDark ? "#e4e4e7" : "#18181b";

    const svg = d3.select(svgRef.current);
    svg.selectAll("*").remove();

    const g = svg.append("g");

    // Zoom
    const zoom = d3.zoom<SVGSVGElement, unknown>()
      .scaleExtent([0.1, 4])
      .on("zoom", (event) => {
        g.attr("transform", event.transform);
      });
    svg.call(zoom);

    // Assign each node a layer based on minimum step index across all paths
    const nodeLayer = new Map<string, number>();
    const activePaths = highlightedPath != null ? [paths[highlightedPath]] : paths;

    for (const path of activePaths) {
      for (let i = 0; i < path.length; i++) {
        const existing = nodeLayer.get(path[i]);
        if (existing === undefined || i < existing) {
          nodeLayer.set(path[i], i);
        }
      }
    }

    // Also include all path nodes even when a specific path is highlighted
    if (highlightedPath == null) {
      for (const path of paths) {
        for (let i = 0; i < path.length; i++) {
          const existing = nodeLayer.get(path[i]);
          if (existing === undefined || i < existing) {
            nodeLayer.set(path[i], i);
          }
        }
      }
    }

    // Group nodes by layer
    const layers = new Map<number, string[]>();
    for (const [nodeKey, layer] of nodeLayer) {
      const list = layers.get(layer) || [];
      list.push(nodeKey);
      layers.set(layer, list);
    }

    // Sort within each layer for determinism
    for (const [, list] of layers) {
      list.sort();
    }

    const maxLayer = Math.max(...Array.from(layers.keys()));
    const layerCount = maxLayer + 1;

    // Layout constants
    const padX = 120;
    const padY = 60;
    const usableWidth = width - padX * 2;
    const usableHeight = height - padY * 2;
    const colSpacing = layerCount > 1 ? usableWidth / (layerCount - 1) : 0;

    // Position nodes
    const nodePositions = new Map<string, { x: number; y: number }>();
    for (const [layer, list] of layers) {
      const x = padX + layer * colSpacing;
      const rowSpacing = list.length > 1 ? usableHeight / (list.length - 1) : 0;
      const yOffset = list.length === 1 ? usableHeight / 2 : 0;
      for (let i = 0; i < list.length; i++) {
        const y = padY + yOffset + i * rowSpacing;
        nodePositions.set(list[i], { x, y });
      }
    }

    // Determine which edges are on highlighted paths
    const highlightedEdges = new Set<string>();
    const highlightedNodes = new Set<string>();
    const targetPaths = highlightedPath != null ? [paths[highlightedPath]] : paths;
    for (const path of targetPaths) {
      for (const n of path) highlightedNodes.add(n);
      for (let i = 0; i < path.length - 1; i++) {
        highlightedEdges.add(`${path[i]}->${path[i + 1]}`);
      }
    }

    // Arrow markers
    const defs = svg.append("defs");
    for (const [id, color] of [["path-edge", "#10b981"], ["context-edge", contextEdgeColor]]) {
      defs
        .append("marker")
        .attr("id", `arrow-${id}`)
        .attr("viewBox", "0 -5 10 10")
        .attr("refX", 18)
        .attr("refY", 0)
        .attr("markerWidth", 6)
        .attr("markerHeight", 6)
        .attr("orient", "auto")
        .append("path")
        .attr("d", "M0,-5L10,0L0,5")
        .attr("fill", color);
    }

    // Draw edges as curves
    const linkGen = d3.linkHorizontal<{ source: [number, number]; target: [number, number] }, [number, number]>()
      .x(d => d[0])
      .y(d => d[1]);

    // Draw context (non-highlighted) edges first
    const edgeData = edges
      .filter(e => nodePositions.has(e.from) && nodePositions.has(e.to))
      .map(e => {
        const key = `${e.from}->${e.to}`;
        return { ...e, isPath: highlightedEdges.has(key) };
      });

    // Context edges
    g.append("g")
      .selectAll("path")
      .data(edgeData.filter(e => !e.isPath))
      .join("path")
      .attr("d", e => {
        const s = nodePositions.get(e.from)!;
        const t = nodePositions.get(e.to)!;
        return linkGen({ source: [s.x, s.y], target: [t.x, t.y] });
      })
      .attr("fill", "none")
      .attr("stroke", contextEdgeColor)
      .attr("stroke-opacity", contextEdgeOpacity)
      .attr("stroke-width", 1)
      .attr("marker-end", "url(#arrow-context-edge)");

    // Path edges (on top)
    g.append("g")
      .selectAll("path")
      .data(edgeData.filter(e => e.isPath))
      .join("path")
      .attr("d", e => {
        const s = nodePositions.get(e.from)!;
        const t = nodePositions.get(e.to)!;
        return linkGen({ source: [s.x, s.y], target: [t.x, t.y] });
      })
      .attr("fill", "none")
      .attr("stroke", "#10b981")
      .attr("stroke-opacity", 0.8)
      .attr("stroke-width", 2.5)
      .attr("marker-end", "url(#arrow-path-edge)");

    // Draw nodes
    const nodeData = Array.from(nodePositions.entries()).map(([key, pos]) => {
      const raw = nodes[key];
      const node = raw ? safeNode(raw) : { key, kind: "", package: "", tags: [], visibility: [], is_test: false, is_external: false };
      return { key, pos, node, isOnPath: highlightedNodes.has(key) };
    });

    const nodeGroup = g.append("g")
      .selectAll<SVGGElement, typeof nodeData[0]>("g")
      .data(nodeData)
      .join("g")
      .attr("transform", d => `translate(${d.pos.x},${d.pos.y})`)
      .style("cursor", "pointer");

    nodeGroup
      .append("circle")
      .attr("r", d => d.isOnPath ? 10 : 7)
      .attr("fill", d => d.node.is_external ? "#94a3b8" : hashColor(d.node.package))
      .attr("stroke", d => d.isOnPath ? "#10b981" : isDark ? "#27272a" : "#fff")
      .attr("stroke-width", d => d.isOnPath ? 3 : 1.5)
      .attr("opacity", d => d.isOnPath ? 1 : 0.6);

    // Labels — show all labels since paths are small
    nodeGroup
      .append("text")
      .text(d => {
        const parts = d.key.split(":");
        return parts[parts.length - 1];
      })
      .attr("font-size", "10px")
      .attr("dx", 14)
      .attr("dy", 4)
      .attr("fill", d => d.isOnPath ? labelBrightColor : labelDimColor)
      .attr("font-weight", d => d.isOnPath ? "600" : "400");

    // Events
    nodeGroup
      .on("mouseenter", (_event, d) => setHoveredNode(d.key))
      .on("mouseleave", () => setHoveredNode(null))
      .on("click", (_event, d) => onNodeClick?.(d.key));

    // Initial zoom to fit
    setTimeout(() => {
      const bounds = g.node()?.getBBox();
      if (bounds) {
        const pad = 40;
        const scale = Math.min(
          width / (bounds.width + pad * 2),
          height / (bounds.height + pad * 2),
          1.5
        );
        const tx = width / 2 - (bounds.x + bounds.width / 2) * scale;
        const ty = height / 2 - (bounds.y + bounds.height / 2) * scale;
        svg
          .transition()
          .duration(500)
          .call(zoom.transform, d3.zoomIdentity.translate(tx, ty).scale(scale));
      }
    }, 100);
  }, [nodes, edges, paths, highlightedPath, onNodeClick, width, height]);

  useEffect(() => {
    renderGraph();
  }, [renderGraph]);

  return (
    <div className="relative h-full w-full">
      <svg
        ref={svgRef}
        width="100%"
        height="100%"
        viewBox={`0 0 ${width} ${height}`}
        preserveAspectRatio="xMidYMid meet"
        className="bg-white dark:bg-zinc-950"
      />

      {hoveredNode && nodes[hoveredNode] && (
        <div className="pointer-events-none absolute left-4 top-4 z-10 rounded-lg border border-zinc-200 bg-white/95 px-3 py-2 text-sm shadow-lg backdrop-blur dark:border-zinc-700 dark:bg-zinc-900/95">
          <div className="font-mono text-xs font-semibold text-zinc-900 dark:text-zinc-100">
            {hoveredNode}
          </div>
          <div className="mt-1 text-xs text-zinc-500">
            {nodes[hoveredNode]?.kind ?? ""} | {nodes[hoveredNode]?.package ?? ""}
          </div>
        </div>
      )}
    </div>
  );
}
