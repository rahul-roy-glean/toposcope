"use client";

import { useEffect, useState } from "react";
import { Sidebar } from "./sidebar";
import type { Repository } from "@/lib/types";
import { getAPI } from "@/lib/api";

export function AppShell({ children }: { children: React.ReactNode }) {
  const [repos, setRepos] = useState<Repository[]>([]);

  useEffect(() => {
    getAPI().then((api) => api.getRepos()).then(setRepos);
  }, []);

  return (
    <div className="flex h-[calc(100vh-3.5rem)]">
      <Sidebar repos={repos} />
      <main className="flex-1 overflow-y-auto">
        {children}
      </main>
    </div>
  );
}
