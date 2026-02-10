"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { GitPullRequest, Package, BarChart3 } from "lucide-react";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { GradeBadge } from "@/components/scoring/grade-badge";
import { Progress } from "@/components/ui/progress";
import { getAPI } from "@/lib/api";
import type { Repository, ScoreResult } from "@/lib/types";

export default function DashboardPage() {
  const [repos, setRepos] = useState<Repository[]>([]);
  const [scores, setScores] = useState<Record<string, ScoreResult[]>>({});
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    async function load() {
      try {
        const api = await getAPI();
        const repoList = await api.getRepos();
        setRepos(repoList);

        const scoreMap: Record<string, ScoreResult[]> = {};
        for (const repo of repoList) {
          try {
            scoreMap[repo.id] = await api.getScores(repo.id);
          } catch {
            scoreMap[repo.id] = [];
          }
        }
        setScores(scoreMap);
      } catch (err) {
        console.error("Failed to load data:", err);
      }
      setLoading(false);
    }
    load();
  }, []);

  if (loading) {
    return (
      <div className="flex h-full items-center justify-center">
        <div className="text-sm text-zinc-400">Loading...</div>
      </div>
    );
  }

  // Collect all recent PRs across repos
  const recentPRs: { repo: Repository; score: ScoreResult }[] = [];
  for (const repo of repos) {
    for (const score of scores[repo.id] ?? []) {
      if (score.pr_number) {
        recentPRs.push({ repo, score });
      }
    }
  }
  recentPRs.sort((a, b) => (b.score.created_at ?? "").localeCompare(a.score.created_at ?? ""));

  // Stats
  const totalPRs = recentPRs.length;
  const avgScore = repos.length > 0
    ? Math.round(
        repos.reduce((sum, repo) => {
          const latest = scores[repo.id]?.[0];
          return sum + (latest?.total_score ?? 0);
        }, 0) / repos.length
      )
    : 0;

  return (
    <div className="p-8">
      <div className="mb-8">
        <h1 className="text-2xl font-bold text-zinc-900 dark:text-zinc-100">Dashboard</h1>
        <p className="mt-1 text-sm text-zinc-500">
          Structural health overview across all repositories
        </p>
      </div>

      {/* Summary Stats */}
      <div className="mb-8 grid grid-cols-3 gap-4">
        <Card>
          <CardContent>
            <div className="flex items-center gap-3">
              <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-emerald-50 dark:bg-emerald-900/20">
                <BarChart3 className="h-5 w-5 text-emerald-600 dark:text-emerald-400" />
              </div>
              <div>
                <p className="text-2xl font-bold text-zinc-900 dark:text-zinc-100">{avgScore}</p>
                <p className="text-xs text-zinc-500">Avg. Health Score</p>
              </div>
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardContent>
            <div className="flex items-center gap-3">
              <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-sky-50 dark:bg-sky-900/20">
                <Package className="h-5 w-5 text-sky-600 dark:text-sky-400" />
              </div>
              <div>
                <p className="text-2xl font-bold text-zinc-900 dark:text-zinc-100">{repos.length}</p>
                <p className="text-xs text-zinc-500">Repositories</p>
              </div>
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardContent>
            <div className="flex items-center gap-3">
              <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-violet-50 dark:bg-violet-900/20">
                <GitPullRequest className="h-5 w-5 text-violet-600 dark:text-violet-400" />
              </div>
              <div>
                <p className="text-2xl font-bold text-zinc-900 dark:text-zinc-100">{totalPRs}</p>
                <p className="text-xs text-zinc-500">PRs Analyzed</p>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>

      <div className="grid grid-cols-2 gap-8">
        {/* Repositories */}
        <Card>
          <CardHeader>
            <CardTitle>Repositories</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="space-y-4">
              {repos.map((repo) => {
                const latest = scores[repo.id]?.[0];
                return (
                  <Link
                    key={repo.id}
                    href={`/repos/${repo.id}`}
                    className="flex items-center gap-4 rounded-lg p-3 transition-colors hover:bg-zinc-50 dark:hover:bg-zinc-900/50"
                  >
                    {latest && <GradeBadge grade={latest.grade} size="sm" />}
                    <div className="flex-1">
                      <p className="text-sm font-semibold text-zinc-900 dark:text-zinc-100">
                        {repo.full_name}
                      </p>
                      <p className="text-xs text-zinc-500">{repo.default_branch}</p>
                    </div>
                    {latest && (
                      <div className="w-24">
                        <div className="mb-1 text-right text-xs font-medium text-zinc-600 dark:text-zinc-400">
                          {latest.total_score}/100
                        </div>
                        <Progress value={latest.total_score} />
                      </div>
                    )}
                  </Link>
                );
              })}
            </div>
          </CardContent>
        </Card>

        {/* Recent PR Analyses */}
        <Card>
          <CardHeader>
            <CardTitle>Recent PR Analyses</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="space-y-3">
              {recentPRs.slice(0, 6).map(({ repo, score }, i) => (
                <Link
                  key={i}
                  href={`/repos/${repo.id}/prs/${score.pr_number}`}
                  className="flex items-center gap-3 rounded-lg p-3 transition-colors hover:bg-zinc-50 dark:hover:bg-zinc-900/50"
                >
                  <GradeBadge grade={score.grade} size="sm" />
                  <div className="flex-1">
                    <p className="text-sm font-medium text-zinc-900 dark:text-zinc-100">
                      {repo.full_name} #{score.pr_number}
                    </p>
                    <p className="text-xs text-zinc-500">
                      {score.delta_stats.impacted_targets} targets impacted |{" "}
                      +{score.delta_stats.added_nodes} / -{score.delta_stats.removed_nodes} nodes
                    </p>
                  </div>
                  <span className="font-mono text-sm font-semibold text-zinc-700 dark:text-zinc-300">
                    {score.total_score}
                  </span>
                </Link>
              ))}
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
