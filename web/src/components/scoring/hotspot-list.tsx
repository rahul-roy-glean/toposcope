import { AlertTriangle } from "lucide-react";
import type { Hotspot } from "@/lib/types";

interface HotspotListProps {
  hotspots: Hotspot[];
}

export function HotspotList({ hotspots }: HotspotListProps) {
  if (!hotspots || hotspots.length === 0) {
    return (
      <div className="rounded-lg border border-zinc-200 px-4 py-6 text-center text-sm text-zinc-400 dark:border-zinc-800">
        No structural hotspots detected
      </div>
    );
  }

  return (
    <div className="space-y-3">
      {hotspots.map((hotspot, i) => (
        <div
          key={i}
          className="flex items-start gap-3 rounded-lg border border-zinc-200 p-4 dark:border-zinc-800"
        >
          <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-amber-500" />
          <div className="flex-1 space-y-1">
            <div className="flex items-center gap-2">
              <code className="rounded bg-zinc-100 px-1.5 py-0.5 text-xs font-medium text-zinc-800 dark:bg-zinc-800 dark:text-zinc-200">
                {hotspot.node_key}
              </code>
              <span className="font-mono text-xs font-semibold text-red-600 dark:text-red-400">
                +{hotspot.score_contribution.toFixed(1)}
              </span>
            </div>
            <div className="flex items-center gap-1.5">
              <span className="text-xs text-zinc-500">Flagged by</span>
              {(hotspot.metric_keys ?? []).map((key) => (
                <span
                  key={key}
                  className="rounded bg-amber-50 px-1.5 py-0.5 text-[10px] font-medium text-amber-700 dark:bg-amber-900/30 dark:text-amber-400"
                >
                  {key.replace(/_/g, " ")}
                </span>
              ))}
            </div>
          </div>
        </div>
      ))}
    </div>
  );
}
