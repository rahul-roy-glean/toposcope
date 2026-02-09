"use client";

import { useEffect, useRef, useState, useCallback } from "react";
import * as d3 from "d3";
import type { PackageNode, PackageEdge } from "@/lib/types";

interface PackageGraphProps {
  nodes: Record<string, PackageNode>;
  edges: PackageEdge[];
  onPackageClick?: (pkg: string) => void;
  highlightedPackage?: string | null;
  minEdgeWeight?: number;
  width?: number;
  height?: number;
}

interface SimNode extends d3.SimulationNodeDatum {
  id: string;
  pkg: PackageNode;
  color: string;
  degree: number;
}

interface SimLink extends d3.SimulationLinkDatum<SimNode> {
  weight: number;
}

function hashColor(str: string): string {
  let hash = 0;
  for (let i = 0; i < str.length; i++) {
    hash = str.charCodeAt(i) + ((hash << 5) - hash);
  }
  const hue = ((hash % 360) + 360) % 360;
  return `hsl(${hue}, 55%, 50%)`;
}

function shortLabel(pkg: string): string {
  // "//go/auth" -> "go/auth", "@external//foo" -> "@external//foo"
  return pkg.replace(/^\/\//, "");
}

export function PackageGraph({
  nodes,
  edges,
  onPackageClick,
  highlightedPackage,
  minEdgeWeight = 1,
  width = 900,
  height = 600,
}: PackageGraphProps) {
  const svgRef = useRef<SVGSVGElement>(null);
  const [hoveredNode, setHoveredNode] = useState<string | null>(null);

  const nodeCount = Object.keys(nodes).length;

  // Filter edges by minimum weight (default weight to 1 if missing)
  const filteredEdges = edges.filter((e) => (e.weight ?? 1) >= minEdgeWeight);

  const renderGraph = useCallback(() => {
    if (!svgRef.current) return;

    // Safety guard: refuse to render massive graphs that would freeze the browser
    if (nodeCount > 1000) return;

    const isDark = document.documentElement.classList.contains("dark");
    const edgeColor = isDark ? "#52525b" : "#94a3b8";
    const edgeOpacity = isDark ? 0.6 : 0.2;
    const edgeHoverOpacity = isDark ? 0.9 : 0.7;
    const edgeDimOpacity = isDark ? 0.15 : 0.1;
    const labelColor = isDark ? "#a1a1aa" : "#71717a";

    const svg = d3.select(svgRef.current);
    svg.selectAll("*").remove();

    const g = svg.append("g");

    // Zoom
    const zoom = d3.zoom<SVGSVGElement, unknown>()
      .scaleExtent([0.05, 4])
      .on("zoom", (event) => {
        g.attr("transform", event.transform);
      });
    svg.call(zoom);

    // Compute degree for each package
    const degreeMap = new Map<string, number>();
    for (const e of filteredEdges) {
      degreeMap.set(e.from, (degreeMap.get(e.from) || 0) + 1);
      degreeMap.set(e.to, (degreeMap.get(e.to) || 0) + 1);
    }

    // Build simulation data
    const simNodes: SimNode[] = Object.entries(nodes).map(([key, pkg]) => ({
      id: key,
      pkg,
      color: pkg.is_external ? "#94a3b8" : hashColor(key.split("/").slice(0, 3).join("/")),
      degree: degreeMap.get(key) || 0,
    }));

    const nodeMap = new Map(simNodes.map((n) => [n.id, n]));

    const simLinks: SimLink[] = filteredEdges
      .filter((e) => nodeMap.has(e.from) && nodeMap.has(e.to))
      .map((e) => ({
        source: nodeMap.get(e.from)!,
        target: nodeMap.get(e.to)!,
        weight: e.weight,
      }));

    // Determine top nodes by degree for label display
    const sortedByDegree = [...simNodes].sort((a, b) => b.degree - a.degree);
    const topLabelSet = new Set(sortedByDegree.slice(0, 30).map((n) => n.id));

    // Simulation
    const chargeStrength = nodeCount > 300 ? -100 : nodeCount > 100 ? -200 : -400;
    const simulation = d3
      .forceSimulation(simNodes)
      .force(
        "link",
        d3
          .forceLink<SimNode, SimLink>(simLinks)
          .id((d) => d.id)
          .distance(60)
      )
      .force("charge", d3.forceManyBody().strength(chargeStrength))
      .force("center", d3.forceCenter(width / 2, height / 2))
      .force("collision", d3.forceCollide((d: SimNode) => Math.sqrt(d.pkg.target_count || 1) * 2 + 5));

    // Arrow marker
    const defs = svg.append("defs");
    defs
      .append("marker")
      .attr("id", "pkg-arrow")
      .attr("viewBox", "0 -5 10 10")
      .attr("refX", 20)
      .attr("refY", 0)
      .attr("markerWidth", 5)
      .attr("markerHeight", 5)
      .attr("orient", "auto")
      .append("path")
      .attr("d", "M0,-5L10,0L0,5")
      .attr("fill", edgeColor);

    // Compute max weight for log scale
    const maxWeight = Math.max(1, ...filteredEdges.map((e) => e.weight));
    const widthScale = d3.scaleLog().domain([1, maxWeight]).range([0.5, 4]);

    // Links
    const link = g
      .append("g")
      .selectAll("line")
      .data(simLinks)
      .join("line")
      .attr("stroke", edgeColor)
      .attr("stroke-opacity", edgeOpacity)
      .attr("stroke-width", (d) => widthScale(d.weight))
      .attr("marker-end", "url(#pkg-arrow)");

    // Node radius: sqrt scale of target count
    const maxTargets = Math.max(1, ...Object.values(nodes).map((n) => n.target_count || 1));
    const radiusScale = d3.scaleSqrt().domain([1, maxTargets]).range([4, 24]);

    // Nodes
    const nodeGroup = g
      .append("g")
      .selectAll<SVGGElement, SimNode>("g")
      .data(simNodes)
      .join("g")
      .style("cursor", "pointer");

    // Drag behavior
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
      .attr("r", (d) => radiusScale(d.pkg.target_count))
      .attr("fill", (d) => d.color)
      .attr("stroke", (d) =>
        highlightedPackage === d.id ? "#f59e0b" : isDark ? "#27272a" : "#fff"
      )
      .attr("stroke-width", (d) =>
        highlightedPackage === d.id ? 3 : 1.5
      )
      .attr("opacity", (d) => (d.pkg.is_external ? 0.5 : 0.85));

    // Labels for top 30 by degree
    nodeGroup
      .append("text")
      .text((d) => shortLabel(d.id))
      .attr("font-size", "8px")
      .attr("dx", (d) => radiusScale(d.pkg.target_count) + 3)
      .attr("dy", 3)
      .attr("fill", labelColor)
      .attr("display", (d) => {
        if (highlightedPackage === d.id) return "block";
        if (topLabelSet.has(d.id)) return "block";
        return "none";
      });

    // Events
    nodeGroup
      .on("mouseenter", (_event, d) => {
        setHoveredNode(d.id);
        // Highlight connected edges
        link.attr("stroke-opacity", (l) => {
          const src = (l.source as SimNode).id;
          const tgt = (l.target as SimNode).id;
          return src === d.id || tgt === d.id ? edgeHoverOpacity : edgeDimOpacity;
        });
      })
      .on("mouseleave", () => {
        setHoveredNode(null);
        link.attr("stroke-opacity", edgeOpacity);
      })
      .on("click", (_event, d) => {
        onPackageClick?.(d.id);
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
      if (bounds && bounds.width > 0) {
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
  }, [nodes, filteredEdges, highlightedPackage, onPackageClick, width, height, nodeCount, minEdgeWeight]);

  useEffect(() => {
    const cleanup = renderGraph();
    return () => cleanup?.();
  }, [renderGraph]);

  const hoveredPkg = hoveredNode ? nodes[hoveredNode] : null;

  if (nodeCount > 1000) {
    return (
      <div className="flex h-full w-full items-center justify-center">
        <div className="text-center">
          <p className="text-sm text-zinc-500">
            Too many packages to render ({nodeCount.toLocaleString()}).
          </p>
          <p className="mt-1 text-xs text-zinc-400">
            Increase the minimum edge weight filter or enable test/external hiding.
          </p>
        </div>
      </div>
    );
  }

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
      {hoveredNode && hoveredPkg && (
        <div className="pointer-events-none absolute left-4 top-4 z-10 rounded-lg border border-zinc-200 bg-white/95 px-3 py-2 text-sm shadow-lg backdrop-blur dark:border-zinc-700 dark:bg-zinc-900/95">
          <div className="font-mono text-xs font-semibold text-zinc-900 dark:text-zinc-100">
            {hoveredNode}
          </div>
          <div className="mt-1 space-y-0.5 text-xs text-zinc-500">
            <div>{hoveredPkg.target_count ?? 0} targets</div>
            <div>{(hoveredPkg.kinds ?? []).join(", ")}</div>
            {hoveredPkg.has_tests && <div>has tests</div>}
          </div>
        </div>
      )}

      {/* Stats */}
      <div className="absolute bottom-4 left-4 rounded-lg border border-zinc-200 bg-white/95 px-3 py-2 text-xs shadow-sm backdrop-blur dark:border-zinc-700 dark:bg-zinc-900/95">
        <span className="text-zinc-500">
          {nodeCount} packages | {filteredEdges.length} edges
        </span>
      </div>
    </div>
  );
}
