import { cn } from "@/lib/cn";

type BadgeVariant = "default" | "success" | "warning" | "danger" | "info" | "muted";

const variantStyles: Record<BadgeVariant, string> = {
  default: "bg-zinc-100 text-zinc-800 dark:bg-zinc-800 dark:text-zinc-200",
  success: "bg-emerald-100 text-emerald-800 dark:bg-emerald-900/40 dark:text-emerald-300",
  warning: "bg-amber-100 text-amber-800 dark:bg-amber-900/40 dark:text-amber-300",
  danger: "bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-300",
  info: "bg-sky-100 text-sky-800 dark:bg-sky-900/40 dark:text-sky-300",
  muted: "bg-zinc-50 text-zinc-500 dark:bg-zinc-900 dark:text-zinc-500",
};

interface BadgeProps {
  variant?: BadgeVariant;
  children: React.ReactNode;
  className?: string;
}

export function Badge({ variant = "default", children, className }: BadgeProps) {
  return (
    <span
      className={cn(
        "inline-flex items-center rounded-md px-2 py-0.5 text-xs font-medium",
        variantStyles[variant],
        className
      )}
    >
      {children}
    </span>
  );
}
