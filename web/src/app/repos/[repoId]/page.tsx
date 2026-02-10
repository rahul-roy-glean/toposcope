"use client";

import { useEffect, useState, useMemo } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import { TrendingUp, TrendingDown, Minus, BarChart3, GitCommit, ExternalLink } from "lucide-react";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { GradeBadge } from "@/components/scoring/grade-badge";
import { ScoreBreakdown } from "@/components/scoring/score-breakdown";
import { HotspotList } from "@/components/scoring/hotspot-list";
import { getAPI } from "@/lib/api";
import type { Repository, ScoreResult } from "@/lib/types";

const GITHUB_BASE = "https://github.com/askscio/scio/commit";

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

  // Find the most recent non-trivial score (score > 0)
  const latestNonTrivial = useMemo(
    () => scores.find((s) => s.total_score > 0) ?? null,
    [scores]
  );
  const latest = scores[0] ?? null;
  const displayScore = latestNonTrivial ?? latest;

  // Compute trend from last few non-zero scores
  const trend = useMemo(() => {
    const nonZero = scores.filter((s) => s.total_score > 0).slice(0, 5);
    if (nonZero.length < 2) return null;
    const recent = nonZero[0].total_score;
    const older = nonZero[nonZero.length - 1].total_score;
    const diff = recent - older;
    return { direction: diff > 1 ? "up" : diff < -1 ? "down" : "flat", diff };
  }, [scores]);

  if (loading) {
    return (
      <div className="flex h-full items-center justify-center">
        <div className="text-sm text-zinc-400">Loading...</div>
      </div>
    );
  }

  return (
    <div className="p-8">
      <div className="mb-8">
        <h1 className="text-2xl font-bold text-zinc-900 dark:text-zinc-100">
          {repo?.full_name ?? params.repoId}
        </h1>
        <p className="mt-1 text-sm text-zinc-500">
          Branch: {repo?.default_branch ?? "main"} | {scores.length} analyses
        </p>
      </div>

      {/* Summary cards */}
      <div className="mb-8 grid grid-cols-4 gap-4">
        {/* Current grade */}
        <Card>
          <CardContent>
            <div className="flex flex-col items-center gap-2 py-2">
              <GradeBadge grade={displayScore?.grade ?? "A"} size="lg" />
              <div className="text-center">
                <p className="text-2xl font-bold text-zinc-900 dark:text-zinc-100">
                  {displayScore?.total_score.toFixed(1) ?? "0.0"}
                </p>
                <p className="text-xs text-zinc-500">
                  {latestNonTrivial ? "Latest Non-trivial Score" : "Latest Score"}
                </p>
              </div>
            </div>
          </CardContent>
        </Card>

        {/* Trend */}
        <Card>
          <CardContent>
            <div className="flex items-center gap-3">
              <div className={`flex h-10 w-10 items-center justify-center rounded-lg ${
                trend?.direction === "up" ? "bg-red-50 dark:bg-red-900/20" :
                trend?.direction === "down" ? "bg-emerald-50 dark:bg-emerald-900/20" :
                "bg-zinc-50 dark:bg-zinc-900/20"
              }`}>
                {trend?.direction === "up" ? (
                  <TrendingUp className="h-5 w-5 text-red-500" />
                ) : trend?.direction === "down" ? (
                  <TrendingDown className="h-5 w-5 text-emerald-500" />
                ) : (
                  <Minus className="h-5 w-5 text-zinc-400" />
                )}
              </div>
              <div>
                <p className="text-lg font-bold text-zinc-900 dark:text-zinc-100">
                  {trend ? `${trend.diff > 0 ? "+" : ""}${trend.diff.toFixed(1)}` : "--"}
                </p>
                <p className="text-xs text-zinc-500">Score Trend</p>
              </div>
            </div>
          </CardContent>
        </Card>

        {/* Delta stats from latest non-trivial */}
        <Card>
          <CardContent>
            <div className="flex items-center gap-3">
              <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-sky-50 dark:bg-sky-900/20">
                <GitCommit className="h-5 w-5 text-sky-600 dark:text-sky-400" />
              </div>
              <div>
                <p className="text-lg font-bold text-zinc-900 dark:text-zinc-100">
                  +{displayScore?.delta_stats?.added_nodes ?? 0} / -{displayScore?.delta_stats?.removed_nodes ?? 0}
                </p>
                <p className="text-xs text-zinc-500">Nodes Changed (latest)</p>
              </div>
            </div>
          </CardContent>
        </Card>

        {/* Link to history */}
        <Link href={`/repos/${params.repoId}/history`}>
          <Card className="h-full cursor-pointer transition-colors hover:bg-zinc-50 dark:hover:bg-zinc-900/50">
            <CardContent>
              <div className="flex items-center gap-3">
                <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-violet-50 dark:bg-violet-900/20">
                  <BarChart3 className="h-5 w-5 text-violet-600 dark:text-violet-400" />
                </div>
                <div>
                  <p className="text-lg font-bold text-zinc-900 dark:text-zinc-100">
                    Score History
                  </p>
                  <p className="text-xs text-zinc-500">Charts, trends & full table</p>
                </div>
              </div>
            </CardContent>
          </Card>
        </Link>
      </div>

      {/* Score breakdown + Hotspots for latest non-trivial */}
      {displayScore && displayScore.total_score > 0 && (
        <div className="mb-8">
          <div className="mb-3 flex items-center gap-2">
            <h2 className="text-lg font-semibold text-zinc-900 dark:text-zinc-100">
              Latest Analysis
            </h2>
            {displayScore.commit_sha && (
              <a
                href={`${GITHUB_BASE}/${displayScore.commit_sha}`}
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center gap-1 font-mono text-xs text-emerald-600 hover:underline dark:text-emerald-400"
              >
                {displayScore.commit_sha.slice(0, 8)}
                <ExternalLink className="h-3 w-3" />
              </a>
            )}
          </div>
          <div className="grid grid-cols-2 gap-8">
            <div>
              <ScoreBreakdown metrics={displayScore.breakdown ?? []} />
            </div>
            <div>
              <h3 className="mb-3 text-lg font-semibold text-zinc-900 dark:text-zinc-100">
                Structural Hotspots
              </h3>
              <HotspotList hotspots={displayScore.hotspots ?? []} />
            </div>
          </div>
        </div>
      )}

      {/* Recent notable scores (non-zero only) */}
      {scores.filter((s) => s.total_score > 0).length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle>Recent Notable Changes</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="space-y-3">
              {scores
                .filter((s) => s.total_score > 0)
                .slice(0, 10)
                .map((score, i) => {
                  const stats = score.delta_stats;
                  return (
                    <Link
                      key={score.id ?? i}
                      href={`/repos/${params.repoId}/scores/${score.id ?? i}`}
                      className="flex items-center gap-3 rounded-lg p-3 transition-colors hover:bg-zinc-50 dark:hover:bg-zinc-900/50"
                    >
                      <GradeBadge grade={score.grade} size="sm" />
                      <div className="flex-1">
                        <p className="text-sm font-medium text-zinc-900 dark:text-zinc-100">
                          {score.commit_sha ? (
                            <span className="font-mono">{score.commit_sha.slice(0, 8)}</span>
                          ) : (
                            score.pr_number ? `PR #${score.pr_number}` : "unknown"
                          )}
                        </p>
                        <p className="text-xs text-zinc-500">
                          {stats ? `+${stats.added_nodes}/-${stats.removed_nodes} nodes` : ""}
                          {score.created_at && ` | ${new Date(score.created_at).toLocaleDateString()}`}
                        </p>
                      </div>
                      <span className="font-mono text-sm font-semibold text-zinc-700 dark:text-zinc-300">
                        {score.total_score.toFixed(1)}
                      </span>
                    </Link>
                  );
                })}
            </div>
          </CardContent>
        </Card>
      )}

      {/* Empty state */}
      {scores.length === 0 && (
        <Card>
          <CardContent>
            <div className="space-y-2 py-4 text-center">
              <p className="text-sm text-zinc-400">No score analyses yet.</p>
              <p className="text-xs text-zinc-400">
                Run <code className="rounded bg-zinc-100 px-1.5 py-0.5 font-mono dark:bg-zinc-800">toposcope score --base main --head HEAD</code> to analyze a change.
              </p>
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  );
}
