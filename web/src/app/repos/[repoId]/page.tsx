"use client";

import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import { Target, GitPullRequest, Package as PackageIcon } from "lucide-react";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { GradeBadge } from "@/components/scoring/grade-badge";
import { Progress } from "@/components/ui/progress";
import { ScoreBreakdown } from "@/components/scoring/score-breakdown";
import { HotspotList } from "@/components/scoring/hotspot-list";
import { getAPI } from "@/lib/api";
import type { Repository, ScoreResult } from "@/lib/types";

export default function RepoOverviewPage() {
  const params = useParams<{ repoId: string }>();
  const [repo, setRepo] = useState<Repository | null>(null);
  const [scores, setScores] = useState<ScoreResult[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    async function load() {
      const api = await getAPI();
      const repos = await api.getRepos();
      const found = repos.find((r) => r.id === params.repoId);
      setRepo(found ?? null);
      const s = await api.getScores(params.repoId);
      setScores(s);
      setLoading(false);
    }
    load();
  }, [params.repoId]);

  if (loading) {
    return (
      <div className="flex h-full items-center justify-center">
        <div className="text-sm text-zinc-400">Loading...</div>
      </div>
    );
  }

  const latest = scores[0];
  const deltaStats = latest?.delta_stats;

  return (
    <div className="p-8">
      <div className="mb-8">
        <h1 className="text-2xl font-bold text-zinc-900 dark:text-zinc-100">
          {repo?.full_name ?? params.repoId}
        </h1>
        <p className="mt-1 text-sm text-zinc-500">
          Branch: {repo?.default_branch ?? "main"}
        </p>
      </div>

      {latest && (
        <>
          {/* Score summary */}
          <div className="mb-8 grid grid-cols-4 gap-4">
            <Card className="col-span-1">
              <CardContent>
                <div className="flex flex-col items-center gap-3 py-2">
                  <GradeBadge grade={latest.grade} size="lg" />
                  <div className="text-center">
                    <p className="text-3xl font-bold text-zinc-900 dark:text-zinc-100">
                      {latest.total_score.toFixed(1)}
                    </p>
                    <p className="text-xs text-zinc-500">Impact Score</p>
                  </div>
                  <Progress value={Math.min(latest.total_score / 30 * 100, 100)} className="mt-1 w-full" />
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardContent>
                <div className="flex items-center gap-3">
                  <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-sky-50 dark:bg-sky-900/20">
                    <Target className="h-5 w-5 text-sky-600 dark:text-sky-400" />
                  </div>
                  <div>
                    <p className="text-2xl font-bold text-zinc-900 dark:text-zinc-100">
                      {deltaStats?.impacted_targets ?? 0}
                    </p>
                    <p className="text-xs text-zinc-500">Impacted Targets</p>
                  </div>
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardContent>
                <div className="flex items-center gap-3">
                  <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-emerald-50 dark:bg-emerald-900/20">
                    <PackageIcon className="h-5 w-5 text-emerald-600 dark:text-emerald-400" />
                  </div>
                  <div>
                    <p className="text-lg font-bold text-zinc-900 dark:text-zinc-100">
                      +{deltaStats?.added_nodes ?? 0} / -{deltaStats?.removed_nodes ?? 0}
                    </p>
                    <p className="text-xs text-zinc-500">Nodes Added/Removed</p>
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
                    <p className="text-lg font-bold text-zinc-900 dark:text-zinc-100">
                      +{deltaStats?.added_edges ?? 0} / -{deltaStats?.removed_edges ?? 0}
                    </p>
                    <p className="text-xs text-zinc-500">Edges Added/Removed</p>
                  </div>
                </div>
              </CardContent>
            </Card>
          </div>

          {/* Score breakdown + Hotspots */}
          <div className="mb-8 grid grid-cols-2 gap-8">
            <div>
              <h2 className="mb-3 text-lg font-semibold text-zinc-900 dark:text-zinc-100">
                Score Breakdown
              </h2>
              <ScoreBreakdown metrics={latest.breakdown ?? []} />
            </div>
            <div>
              <h2 className="mb-3 text-lg font-semibold text-zinc-900 dark:text-zinc-100">
                Structural Hotspots
              </h2>
              <HotspotList hotspots={latest.hotspots ?? []} />
            </div>
          </div>
        </>
      )}

      {/* Score Analyses */}
      <Card>
        <CardHeader>
          <CardTitle>Score Analyses</CardTitle>
        </CardHeader>
        <CardContent>
          {scores.length === 0 ? (
            <div className="space-y-2">
              <p className="text-sm text-zinc-400">No score analyses yet.</p>
              <p className="text-xs text-zinc-400">
                Run <code className="rounded bg-zinc-100 px-1.5 py-0.5 font-mono dark:bg-zinc-800">toposcope score --base main --head HEAD</code> to analyze a change.
              </p>
            </div>
          ) : (
            <div className="space-y-3">
              {scores.map((score, i) => {
                const href = score.pr_number
                  ? `/repos/${params.repoId}/prs/${score.pr_number}`
                  : `/repos/${params.repoId}/scores/${score.id ?? i}`;
                const stats = score.delta_stats;
                return (
                  <Link
                    key={score.id ?? i}
                    href={href}
                    className="flex items-center gap-3 rounded-lg p-3 transition-colors hover:bg-zinc-50 dark:hover:bg-zinc-900/50"
                  >
                    <GradeBadge grade={score.grade} size="sm" />
                    <div className="flex-1">
                      <p className="text-sm font-medium text-zinc-900 dark:text-zinc-100">
                        {score.pr_number ? `PR #${score.pr_number}` : `${(score.base_commit ?? "").slice(0, 7)}..${(score.head_commit ?? "").slice(0, 7)}`}
                      </p>
                      <p className="text-xs text-zinc-500">
                        {stats?.impacted_targets ?? 0} targets impacted
                        {score.analyzed_at && ` | ${new Date(score.analyzed_at).toLocaleDateString()}`}
                      </p>
                    </div>
                    <span className="font-mono text-sm font-semibold text-zinc-700 dark:text-zinc-300">
                      {score.total_score.toFixed(1)}
                    </span>
                  </Link>
                );
              })}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
