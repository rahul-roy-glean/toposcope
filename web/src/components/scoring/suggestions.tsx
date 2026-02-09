import { Lightbulb } from "lucide-react";
import type { SuggestedAction } from "@/lib/types";

interface SuggestionsProps {
  actions: SuggestedAction[];
}

export function Suggestions({ actions }: SuggestionsProps) {
  if (!actions || actions.length === 0) {
    return (
      <div className="rounded-lg border border-zinc-200 px-4 py-6 text-center text-sm text-zinc-400 dark:border-zinc-800">
        No suggested actions
      </div>
    );
  }

  return (
    <div className="space-y-3">
      {actions.map((action, i) => (
        <div
          key={i}
          className="rounded-lg border border-zinc-200 p-4 dark:border-zinc-800"
        >
          <div className="mb-2 flex items-start gap-2">
            <Lightbulb className="mt-0.5 h-4 w-4 shrink-0 text-amber-400" />
            <div className="flex-1">
              <div className="flex items-center justify-between">
                <h4 className="text-sm font-semibold text-zinc-900 dark:text-zinc-100">
                  {action.title}
                </h4>
                <span className="rounded-full bg-zinc-100 px-2 py-0.5 text-[10px] font-medium text-zinc-600 dark:bg-zinc-800 dark:text-zinc-400">
                  {Math.round(action.confidence * 100)}% confidence
                </span>
              </div>
              <p className="mt-1 text-sm text-zinc-600 dark:text-zinc-400">
                {action.description}
              </p>
            </div>
          </div>

          <div className="mt-3 flex flex-wrap gap-1">
            {(action.targets ?? []).map((target) => (
              <code
                key={target}
                className="rounded bg-zinc-100 px-1.5 py-0.5 text-[11px] font-medium text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300"
              >
                {target}
              </code>
            ))}
          </div>

          {(action.addresses ?? []).length > 0 && (
            <div className="mt-2 flex gap-1">
              <span className="text-[10px] text-zinc-400">Addresses:</span>
              {(action.addresses ?? []).map((addr) => (
                <span
                  key={addr}
                  className="rounded bg-emerald-50 px-1.5 py-0.5 text-[10px] font-mono text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400"
                >
                  {addr}
                </span>
              ))}
            </div>
          )}
        </div>
      ))}
    </div>
  );
}
