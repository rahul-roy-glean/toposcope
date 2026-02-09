import { cn } from "@/lib/cn";

interface CardProps {
  children: React.ReactNode;
  className?: string;
}

export function Card({ children, className }: CardProps) {
  return (
    <div
      className={cn(
        "rounded-xl border border-zinc-200 bg-white p-6 shadow-sm dark:border-zinc-800 dark:bg-zinc-950",
        className
      )}
    >
      {children}
    </div>
  );
}

export function CardHeader({ children, className }: CardProps) {
  return (
    <div className={cn("mb-4", className)}>
      {children}
    </div>
  );
}

export function CardTitle({ children, className }: CardProps) {
  return (
    <h3 className={cn("text-lg font-semibold text-zinc-900 dark:text-zinc-100", className)}>
      {children}
    </h3>
  );
}

export function CardDescription({ children, className }: CardProps) {
  return (
    <p className={cn("text-sm text-zinc-500 dark:text-zinc-400", className)}>
      {children}
    </p>
  );
}

export function CardContent({ children, className }: CardProps) {
  return (
    <div className={cn("", className)}>
      {children}
    </div>
  );
}
