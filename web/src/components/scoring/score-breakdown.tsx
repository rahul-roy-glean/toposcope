"use client";

import { useState } from "react";
import { ChevronDown, ChevronRight } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import type { MetricResult, Severity } from "@/lib/types";

function severityVariant(severity: Severity) {
  switch (severity) {
    case "HIGH":
      return "danger" as const;
    case "MEDIUM":
      return "warning" as const;
    case "LOW":
      return "info" as const;
    case "INFO":
      return "muted" as const;
  }
}

interface MetricRowProps {
  metric: MetricResult;
}

function MetricRow({ metric }: MetricRowProps) {
  const [expanded, setExpanded] = useState(false);
  const hasEvidence = (metric.evidence ?? []).length > 0;

  return (
    <div className="border-b border-zinc-100 last:border-0 dark:border-zinc-800">
      <button
        onClick={() => hasEvidence && setExpanded(!expanded)}
        className="flex w-full items-center gap-3 px-4 py-3 text-left hover:bg-zinc-50 dark:hover:bg-zinc-900/50"
        disabled={!hasEvidence}
      >
        <span className="text-zinc-400">
          {hasEvidence ? (
            expanded ? (
              <ChevronDown className="h-4 w-4" />
            ) : (
              <ChevronRight className="h-4 w-4" />
            )
          ) : (
            <span className="inline-block h-4 w-4" />
          )}
        </span>
        <span className="flex-1 text-sm font-medium text-zinc-900 dark:text-zinc-100">
          {metric.name}
        </span>
        <Badge variant={severityVariant(metric.severity)}>{metric.severity}</Badge>
        <span
          className={`min-w-[3rem] text-right font-mono text-sm font-semibold ${
            metric.contribution < 0
              ? "text-red-600 dark:text-red-400"
              : metric.contribution > 0
              ? "text-emerald-600 dark:text-emerald-400"
              : "text-zinc-400"
          }`}
        >
          {metric.contribution > 0 ? "+" : ""}
          {metric.contribution.toFixed(1)}
        </span>
      </button>

      {expanded && hasEvidence && (
        <div className="border-t border-zinc-100 bg-zinc-50/50 px-4 py-3 dark:border-zinc-800 dark:bg-zinc-900/30">
          <ul className="space-y-2">
            {(metric.evidence ?? []).map((ev, i) => (
              <li key={i} className="flex items-start gap-2 text-sm text-zinc-600 dark:text-zinc-400">
                <span className="mt-1 h-1.5 w-1.5 shrink-0 rounded-full bg-zinc-400" />
                <span>{ev.summary}</span>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}

interface ScoreBreakdownProps {
  metrics: MetricResult[];
}

export function ScoreBreakdown({ metrics }: ScoreBreakdownProps) {
  return (
    <div className="overflow-hidden rounded-lg border border-zinc-200 dark:border-zinc-800">
      <div className="border-b border-zinc-200 bg-zinc-50 px-4 py-2 dark:border-zinc-800 dark:bg-zinc-900">
        <h4 className="text-sm font-semibold text-zinc-700 dark:text-zinc-300">
          Score Breakdown
        </h4>
      </div>
      <div>
        {metrics.map((metric) => (
          <MetricRow key={metric.key} metric={metric} />
        ))}
      </div>
    </div>
  );
}
