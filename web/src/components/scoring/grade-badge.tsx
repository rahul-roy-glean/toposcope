import { cn } from "@/lib/cn";

interface GradeBadgeProps {
  grade: string;
  size?: "sm" | "md" | "lg";
  className?: string;
}

function gradeColor(grade: string): string {
  switch (grade) {
    case "A":
      return "bg-emerald-500 text-white";
    case "B":
      return "bg-emerald-400 text-white";
    case "C":
      return "bg-amber-400 text-white";
    case "D":
      return "bg-orange-500 text-white";
    case "F":
      return "bg-red-500 text-white";
    default:
      return "bg-zinc-400 text-white";
  }
}

const sizeStyles = {
  sm: "h-8 w-8 text-sm",
  md: "h-12 w-12 text-xl",
  lg: "h-20 w-20 text-4xl",
};

export function GradeBadge({ grade, size = "md", className }: GradeBadgeProps) {
  return (
    <div
      className={cn(
        "inline-flex items-center justify-center rounded-xl font-bold shadow-sm",
        gradeColor(grade),
        sizeStyles[size],
        className
      )}
    >
      {grade}
    </div>
  );
}
