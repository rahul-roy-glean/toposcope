import Link from "next/link";
import { Hexagon } from "lucide-react";

export function Header() {
  return (
    <header className="sticky top-0 z-50 border-b border-zinc-200 bg-white/80 backdrop-blur dark:border-zinc-800 dark:bg-zinc-950/80">
      <div className="flex h-14 items-center gap-4 px-6">
        <Link href="/" className="flex items-center gap-2">
          <Hexagon className="h-6 w-6 text-emerald-500" />
          <span className="text-lg font-bold tracking-tight text-zinc-900 dark:text-zinc-100">
            Toposcope
          </span>
        </Link>

        <nav className="ml-8 flex items-center gap-6">
          <Link
            href="/"
            className="text-sm font-medium text-zinc-600 transition-colors hover:text-zinc-900 dark:text-zinc-400 dark:hover:text-zinc-100"
          >
            Dashboard
          </Link>
        </nav>

        <div className="ml-auto flex items-center gap-2">
          <span className="rounded-full bg-emerald-100 px-2.5 py-0.5 text-xs font-medium text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400">
            Structural Intelligence
          </span>
        </div>
      </div>
    </header>
  );
}
