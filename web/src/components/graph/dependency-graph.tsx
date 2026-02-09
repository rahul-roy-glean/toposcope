"use client";

import { useEffect, useRef, useState, useCallback } from "react";
import * as d3 from "d3";
import type { Node, Edge } from "@/lib/types";

interface DependencyGraphProps {
  nodes: Record<string, Node>;
  edges: Edge[];
  highlightedNodes?: Set<string>;
  addedEdges?: Set<string>;
  removedEdges?: Set<string>;
  onNodeClick?: (nodeKey: string) => void;
  width?: number;
  height?: number;
}

interface SimNode extends d3.SimulationNodeDatum {
  id: string;
  node: Node;
  color: string;
}

interface SimLink extends d3.SimulationLinkDatum<SimNode> {
  edge: Edge;
  added?: boolean;
  removed?: boolean;
}

function hashColor(str: string): string {
  let hash = 0;
  for (let i = 0; i < str.length; i++) {
    hash = str.charCodeAt(i) + ((hash << 5) - hash);
  }
  const hue = ((hash % 360) + 360) % 360;
  return `hsl(${hue}, 55%, 50%)`;
}

function getPackages(nodes: Record<string, Node>): string[] {
  const pkgs = new Set<string>();
  for (const n of Object.values(nodes)) {
    if (n && !n.is_external && n.package) pkgs.add(n.package);
  }
  return Array.from(pkgs).sort();
}

// Ensure node fields are safe from Go's omitempty null serialization
function safeNode(n: Node): Node {
  return {
    ...n,
    tags: n.tags ?? [],
    visibility: n.visibility ?? [],
    package: n.package ?? "",
    kind: n.kind ?? "",
  };
}

export function DependencyGraph({
  nodes,
  edges,
  highlightedNodes,
  addedEdges,
  removedEdges,
  onNodeClick,
  width = 900,
  height = 600,
}: DependencyGraphProps) {
  const svgRef = useRef<SVGSVGElement>(null);
  const [hoveredNode, setHoveredNode] = useState<string | null>(null);

  const nodeCount = Object.keys(nodes).length;
  const packages = getPackages(nodes);

  const renderGraph = useCallback(() => {
    if (!svgRef.current) return;

    const isDark = document.documentElement.classList.contains("dark");
    const edgeColor = isDark ? "#52525b" : "#94a3b8";
    const edgeOpacity = isDark ? 0.7 : 0.3;
    const labelColor = isDark ? "#a1a1aa" : "#71717a";

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

    // Build simulation data
    const simNodes: SimNode[] = Object.entries(nodes).map(([key, raw]) => {
      const node = safeNode(raw);
      return {
        id: key,
        node,
        color: node.is_external ? "#94a3b8" : hashColor(node.package),
      };
    });

    const nodeMap = new Map(simNodes.map((n) => [n.id, n]));

    const simLinks: SimLink[] = edges
      .filter((e) => nodeMap.has(e.from) && nodeMap.has(e.to))
      .map((e) => {
        const edgeKey = `${e.from}->${e.to}`;
        return {
          source: nodeMap.get(e.from)!,
          target: nodeMap.get(e.to)!,
          edge: e,
          added: addedEdges?.has(edgeKey),
          removed: removedEdges?.has(edgeKey),
        };
      });

    // Simulation
    const simulation = d3
      .forceSimulation(simNodes)
      .force(
        "link",
        d3
          .forceLink<SimNode, SimLink>(simLinks)
          .id((d) => d.id)
          .distance(80)
      )
      .force("charge", d3.forceManyBody().strength(nodeCount > 100 ? -200 : -400))
      .force("center", d3.forceCenter(width / 2, height / 2))
      .force("collision", d3.forceCollide(20));

    // Arrow markers
    const defs = svg.append("defs");
    for (const color of [edgeColor, "#22c55e", "#ef4444"]) {
      defs
        .append("marker")
        .attr("id", `arrow-${color.replace("#", "")}`)
        .attr("viewBox", "0 -5 10 10")
        .attr("refX", 20)
        .attr("refY", 0)
        .attr("markerWidth", 6)
        .attr("markerHeight", 6)
        .attr("orient", "auto")
        .append("path")
        .attr("d", "M0,-5L10,0L0,5")
        .attr("fill", color);
    }

    // Links
    const link = g
      .append("g")
      .selectAll("line")
      .data(simLinks)
      .join("line")
      .attr("stroke", (d) =>
        d.added ? "#22c55e" : d.removed ? "#ef4444" : edgeColor
      )
      .attr("stroke-opacity", (d) => (d.added || d.removed ? 0.8 : edgeOpacity))
      .attr("stroke-width", (d) => (d.added || d.removed ? 2 : 1))
      .attr("marker-end", (d) => {
        const c = d.added ? "22c55e" : d.removed ? "ef4444" : edgeColor.replace("#", "");
        return `url(#arrow-${c})`;
      });

    // Nodes
    const nodeGroup = g
      .append("g")
      .selectAll<SVGGElement, SimNode>("g")
      .data(simNodes)
      .join("g")
      .style("cursor", "pointer");

    // Apply drag behavior
    const dragBehavior = d3
      .drag<SVGGElement, SimNode>()
      .on("start", (event, d) => {
        if (!event.active) simulation.alphaTarget(0.3).restart();
        d.fx = d.x;
        d.fy = d.y;
      })
      .on("drag", (event, d) => {
        d.fx = event.x;
        d.fy = event.y;
      })
      .on("end", (event, d) => {
        if (!event.active) simulation.alphaTarget(0);
        d.fx = null;
        d.fy = null;
      });

    nodeGroup.call(dragBehavior);

    nodeGroup
      .append("circle")
      .attr("r", (d) => {
        if (d.node.is_test) return 5;
        if (d.node.is_external) return 6;
        if (d.node.kind === "go_binary") return 10;
        return 8;
      })
      .attr("fill", (d) => d.color)
      .attr("stroke", (d) =>
        highlightedNodes?.has(d.id)
          ? "#f59e0b"
          : isDark ? "#27272a" : "#fff"
      )
      .attr("stroke-width", (d) =>
        highlightedNodes?.has(d.id) ? 3 : 1.5
      )
      .attr("opacity", (d) => (d.node.is_test ? 0.6 : 1));

    // Labels (show for binaries and highlighted)
    nodeGroup
      .append("text")
      .text((d) => {
        const parts = d.id.split(":");
        return parts[parts.length - 1];
      })
      .attr("font-size", "9px")
      .attr("dx", 12)
      .attr("dy", 3)
      .attr("fill", labelColor)
      .attr("display", (d) => {
        if (d.node.kind === "go_binary" || highlightedNodes?.has(d.id)) return "block";
        if (nodeCount <= 30) return "block";
        return "none";
      });

    // Events
    nodeGroup
      .on("mouseenter", (_event, d) => {
        setHoveredNode(d.id);
      })
      .on("mouseleave", () => {
        setHoveredNode(null);
      })
      .on("click", (_event, d) => {
        onNodeClick?.(d.id);
      });

    // Tick
    simulation.on("tick", () => {
      link
        .attr("x1", (d) => (d.source as SimNode).x!)
        .attr("y1", (d) => (d.source as SimNode).y!)
        .attr("x2", (d) => (d.target as SimNode).x!)
        .attr("y2", (d) => (d.target as SimNode).y!);

      nodeGroup.attr("transform", (d) => `translate(${d.x},${d.y})`);
    });

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
          .call(
            zoom.transform,
            d3.zoomIdentity.translate(tx, ty).scale(scale)
          );
      }
    }, 1000);

    return () => {
      simulation.stop();
    };
  }, [nodes, edges, highlightedNodes, addedEdges, removedEdges, onNodeClick, width, height, nodeCount]);

  useEffect(() => {
    const cleanup = renderGraph();
    return () => cleanup?.();
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

      {/* Tooltip */}
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

      {/* Legend */}
      <div className="absolute bottom-4 right-4 rounded-lg border border-zinc-200 bg-white/95 px-3 py-2 text-xs shadow-sm backdrop-blur dark:border-zinc-700 dark:bg-zinc-900/95">
        <div className="mb-1 font-semibold text-zinc-600 dark:text-zinc-400">Packages</div>
        <div className="flex max-w-[200px] flex-wrap gap-x-3 gap-y-1">
          {packages.slice(0, 8).map((pkg) => (
            <div key={pkg} className="flex items-center gap-1">
              <span
                className="inline-block h-2.5 w-2.5 rounded-full"
                style={{ backgroundColor: hashColor(pkg) }}
              />
              <span className="text-zinc-500 dark:text-zinc-400">
                {pkg.replace("//", "")}
              </span>
            </div>
          ))}
          {packages.length > 8 && (
            <span className="text-zinc-400">+{packages.length - 8} more</span>
          )}
        </div>
        <div className="mt-1.5 flex gap-3 border-t border-zinc-100 pt-1.5 dark:border-zinc-800">
          <div className="flex items-center gap-1">
            <span className="inline-block h-2.5 w-2.5 rounded-full bg-slate-400" />
            <span className="text-zinc-500">external</span>
          </div>
        </div>
      </div>
    </div>
  );
}
