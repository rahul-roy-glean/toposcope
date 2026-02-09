"use client";

import { X, ArrowDownRight, ArrowUpLeft } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import type { Node, Edge } from "@/lib/types";

interface NodeDetailProps {
  nodeKey: string;
  node: Node;
  edges: Edge[];
  allNodes: Record<string, Node>;
  onClose: () => void;
  onNavigate?: (nodeKey: string) => void;
}

export function NodeDetail({ nodeKey, node, edges, onClose, onNavigate }: NodeDetailProps) {
  const inbound = edges.filter((e) => e.to === nodeKey);
  const outbound = edges.filter((e) => e.from === nodeKey);

  return (
    <div className="flex h-full flex-col border-l border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-950">
      <div className="flex items-center justify-between border-b border-zinc-200 px-4 py-3 dark:border-zinc-800">
        <h3 className="text-sm font-semibold text-zinc-900 dark:text-zinc-100">Node Details</h3>
        <button
          onClick={onClose}
          className="rounded p-1 hover:bg-zinc-100 dark:hover:bg-zinc-800"
        >
          <X className="h-4 w-4 text-zinc-500" />
        </button>
      </div>

      <div className="flex-1 overflow-y-auto p-4">
        <div className="space-y-4">
          {/* Key */}
          <div>
            <label className="text-[10px] font-medium uppercase tracking-wider text-zinc-400">
              Target
            </label>
            <p className="mt-0.5 break-all font-mono text-sm font-semibold text-zinc-900 dark:text-zinc-100">
              {nodeKey}
            </p>
          </div>

          {/* Kind + Package */}
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="text-[10px] font-medium uppercase tracking-wider text-zinc-400">
                Kind
              </label>
              <p className="mt-0.5 text-sm text-zinc-700 dark:text-zinc-300">{node.kind}</p>
            </div>
            <div>
              <label className="text-[10px] font-medium uppercase tracking-wider text-zinc-400">
                Package
              </label>
              <p className="mt-0.5 text-sm text-zinc-700 dark:text-zinc-300">{node.package}</p>
            </div>
          </div>

          {/* Flags */}
          <div className="flex flex-wrap gap-2">
            {node.is_test && <Badge variant="info">test</Badge>}
            {node.is_external && <Badge variant="muted">external</Badge>}
            {(node.tags ?? []).map((tag) => (
              <Badge key={tag} variant="default">{tag}</Badge>
            ))}
          </div>

          {/* Visibility */}
          {(node.visibility ?? []).length > 0 && (
            <div>
              <label className="text-[10px] font-medium uppercase tracking-wider text-zinc-400">
                Visibility
              </label>
              <div className="mt-1 space-y-0.5">
                {(node.visibility ?? []).map((v) => (
                  <p key={v} className="font-mono text-xs text-zinc-600 dark:text-zinc-400">
                    {v}
                  </p>
                ))}
              </div>
            </div>
          )}

          {/* Degree */}
          <div className="grid grid-cols-2 gap-3 rounded-lg bg-zinc-50 p-3 dark:bg-zinc-900">
            <div className="text-center">
              <div className="text-2xl font-bold text-zinc-900 dark:text-zinc-100">
                {inbound.length}
              </div>
              <div className="text-[10px] font-medium uppercase tracking-wider text-zinc-400">
                In-Degree
              </div>
            </div>
            <div className="text-center">
              <div className="text-2xl font-bold text-zinc-900 dark:text-zinc-100">
                {outbound.length}
              </div>
              <div className="text-[10px] font-medium uppercase tracking-wider text-zinc-400">
                Out-Degree
              </div>
            </div>
          </div>

          {/* Dependents (in-edges) */}
          {inbound.length > 0 && (
            <div>
              <label className="flex items-center gap-1 text-[10px] font-medium uppercase tracking-wider text-zinc-400">
                <ArrowDownRight className="h-3 w-3" />
                Dependents ({inbound.length})
              </label>
              <ul className="mt-1 space-y-1">
                {inbound.map((e) => (
                  <li key={e.from}>
                    <button
                      onClick={() => onNavigate?.(e.from)}
                      className="w-full rounded px-2 py-1 text-left font-mono text-xs text-zinc-700 hover:bg-zinc-100 dark:text-zinc-300 dark:hover:bg-zinc-800"
                    >
                      {e.from}
                      <span className="ml-1 text-zinc-400">({e.type})</span>
                    </button>
                  </li>
                ))}
              </ul>
            </div>
          )}

          {/* Dependencies (out-edges) */}
          {outbound.length > 0 && (
            <div>
              <label className="flex items-center gap-1 text-[10px] font-medium uppercase tracking-wider text-zinc-400">
                <ArrowUpLeft className="h-3 w-3" />
                Dependencies ({outbound.length})
              </label>
              <ul className="mt-1 space-y-1">
                {outbound.map((e) => (
                  <li key={e.to}>
                    <button
                      onClick={() => onNavigate?.(e.to)}
                      className="w-full rounded px-2 py-1 text-left font-mono text-xs text-zinc-700 hover:bg-zinc-100 dark:text-zinc-300 dark:hover:bg-zinc-800"
                    >
                      {e.to}
                      <span className="ml-1 text-zinc-400">({e.type})</span>
                    </button>
                  </li>
                ))}
              </ul>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
