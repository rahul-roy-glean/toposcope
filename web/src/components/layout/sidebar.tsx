"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { GitBranch, BarChart3, Network, History } from "lucide-react";
import { cn } from "@/lib/cn";
import type { Repository } from "@/lib/types";

interface SidebarProps {
  repos: Repository[];
}

export function Sidebar({ repos }: SidebarProps) {
  const pathname = usePathname();

  return (
    <aside className="flex h-full w-60 flex-col border-r border-zinc-200 bg-zinc-50/50 dark:border-zinc-800 dark:bg-zinc-950/50">
      <div className="p-4">
        <h2 className="text-[10px] font-semibold uppercase tracking-wider text-zinc-400">
          Repositories
        </h2>
      </div>
      <nav className="flex-1 overflow-y-auto px-2">
        {repos.map((repo) => {
          const repoPath = `/repos/${repo.id}`;
          const isActive = pathname.startsWith(repoPath);

          return (
            <div key={repo.id} className="mb-1">
              <Link
                href={repoPath}
                className={cn(
                  "flex items-center gap-2 rounded-lg px-3 py-2 text-sm font-medium transition-colors",
                  isActive
                    ? "bg-zinc-200/80 text-zinc-900 dark:bg-zinc-800 dark:text-zinc-100"
                    : "text-zinc-600 hover:bg-zinc-100 hover:text-zinc-900 dark:text-zinc-400 dark:hover:bg-zinc-800/50 dark:hover:text-zinc-100"
                )}
              >
                <GitBranch className="h-4 w-4 shrink-0" />
                <span className="truncate">{repo.full_name}</span>
              </Link>

              {isActive && (
                <div className="ml-5 mt-0.5 space-y-0.5 border-l border-zinc-200 pl-3 dark:border-zinc-700">
                  <SidebarLink
                    href={repoPath}
                    icon={<BarChart3 className="h-3.5 w-3.5" />}
                    label="Overview"
                    active={pathname === repoPath}
                  />
                  <SidebarLink
                    href={`${repoPath}/graph`}
                    icon={<Network className="h-3.5 w-3.5" />}
                    label="Graph Explorer"
                    active={pathname === `${repoPath}/graph`}
                  />
                  <SidebarLink
                    href={`${repoPath}/history`}
                    icon={<History className="h-3.5 w-3.5" />}
                    label="Score History"
                    active={pathname === `${repoPath}/history`}
                  />
                </div>
              )}
            </div>
          );
        })}
      </nav>
    </aside>
  );
}

function SidebarLink({
  href,
  icon,
  label,
  active,
}: {
  href: string;
  icon: React.ReactNode;
  label: string;
  active: boolean;
}) {
  return (
    <Link
      href={href}
      className={cn(
        "flex items-center gap-2 rounded-md px-2 py-1.5 text-xs font-medium transition-colors",
        active
          ? "text-zinc-900 dark:text-zinc-100"
          : "text-zinc-500 hover:text-zinc-700 dark:text-zinc-500 dark:hover:text-zinc-300"
      )}
    >
      {icon}
      {label}
    </Link>
  );
}
