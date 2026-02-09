import { cn } from "@/lib/cn";

interface ProgressProps {
  value: number;
  max?: number;
  className?: string;
  barClassName?: string;
}

function getColorForScore(score: number): string {
  if (score >= 90) return "bg-emerald-500";
  if (score >= 80) return "bg-emerald-400";
  if (score >= 70) return "bg-amber-400";
  if (score >= 60) return "bg-orange-400";
  return "bg-red-500";
}

export function Progress({ value, max = 100, className, barClassName }: ProgressProps) {
  const pct = Math.min(100, Math.max(0, (value / max) * 100));

  return (
    <div
      className={cn(
        "h-2 w-full overflow-hidden rounded-full bg-zinc-200 dark:bg-zinc-800",
        className
      )}
    >
      <div
        className={cn(
          "h-full rounded-full transition-all duration-500",
          barClassName || getColorForScore(value),
        )}
        style={{ width: `${pct}%` }}
      />
    </div>
  );
}
