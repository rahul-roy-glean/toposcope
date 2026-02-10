"use client";

import { useEffect, useState, useMemo } from "react";
import { useParams } from "next/navigation";
import {
  LineChart,
  Line,
  AreaChart,
  Area,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  Legend,
  ReferenceLine,
} from "recharts";
import { ChevronLeft, ChevronRight, ExternalLink } from "lucide-react";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { GradeBadge } from "@/components/scoring/grade-badge";
import { getAPI } from "@/lib/api";
import type { ScoreHistory } from "@/lib/types";

const METRIC_COLORS: Record<string, string> = {
  m1_fan_in: "#ef4444",
  m2_fan_out: "#f97316",
  m3_dep_depth: "#eab308",
  m4_visibility: "#3b82f6",
  m5_cycle: "#8b5cf6",
  m6_churn: "#ec4899",
};

const METRIC_NAMES: Record<string, string> = {
  m1_fan_in: "Fan-In",
  m2_fan_out: "Fan-Out",
  m3_dep_depth: "Dep Depth",
  m4_visibility: "Visibility",
  m5_cycle: "Cycles",
  m6_churn: "Churn",
};

const PAGE_SIZE = 25;
const GITHUB_BASE = "https://github.com/askscio/scio/commit";

// Compute the 95th percentile for Y-axis capping
function percentile(values: number[], p: number): number {
  const sorted = [...values].sort((a, b) => a - b);
  const idx = Math.ceil((p / 100) * sorted.length) - 1;
  return sorted[Math.max(0, idx)] ?? 0;
}

export default function ScoreHistoryPage() {
  const params = useParams<{ repoId: string }>();
  const [history, setHistory] = useState<ScoreHistory[]>([]);
  const [loading, setLoading] = useState(true);
  const [page, setPage] = useState(0);
  const [gradeFilter, setGradeFilter] = useState<string | null>(null);

  useEffect(() => {
    async function load() {
      const api = await getAPI();
      const h = await api.getScoreHistory(params.repoId);
      setHistory(h);
      setLoading(false);
    }
    load();
  }, [params.repoId]);

  // Check if dates are all the same (needs commit-based X-axis)
  const allSameDate = useMemo(() => {
    if (history.length < 2) return false;
    return history.every((h) => h.date === history[0].date);
  }, [history]);

  // Prepare chart data with an index and short SHA for X-axis
  const chartData = useMemo(() => {
    return history.map((h, i) => ({
      ...h,
      index: i,
      label: allSameDate
        ? (h.commit_sha ?? "").slice(0, 6)
        : h.date.slice(5), // MM-DD
    }));
  }, [history, allSameDate]);

  // Y-axis domain: cap at 95th percentile to handle outliers
  const scoreYMax = useMemo(() => {
    const scores = history.map((h) => h.total_score).filter((s) => s > 0);
    if (scores.length === 0) return 100;
    const p95 = percentile(scores, 95);
    return Math.ceil(p95 * 1.2); // 20% headroom
  }, [history]);

  const metricYMax = useMemo(() => {
    const values: number[] = [];
    for (const h of history) {
      for (const v of Object.values(h.metrics)) {
        values.push(Math.abs(v));
      }
    }
    if (values.length === 0) return 10;
    const p95 = percentile(values, 95);
    return Math.ceil(p95 * 1.2);
  }, [history]);

  // Filtered + reversed (newest first) for table
  const filtered = useMemo(() => {
    const reversed = [...history].reverse();
    if (!gradeFilter) return reversed;
    return reversed.filter((h) => h.grade === gradeFilter);
  }, [history, gradeFilter]);

  // Pagination
  const totalPages = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE));
  const pageData = useMemo(
    () => filtered.slice(page * PAGE_SIZE, (page + 1) * PAGE_SIZE),
    [filtered, page]
  );

  // Available grades for filter
  const availableGrades = useMemo(() => {
    const grades = new Set(history.map((h) => h.grade));
    return ["A", "B", "C", "D", "F"].filter((g) => grades.has(g));
  }, [history]);

  // Reset page when filter changes
  useEffect(() => {
    setPage(0);
  }, [gradeFilter]);

  if (loading) {
    return (
      <div className="flex h-full items-center justify-center">
        <div className="text-sm text-zinc-400">Loading...</div>
      </div>
    );
  }

  // Transform data for the stacked area chart (make contributions absolute)
  const metricData = chartData.map((h) => {
    const entry: Record<string, number | string> = { label: h.label, index: h.index };
    for (const [key, value] of Object.entries(h.metrics)) {
      entry[key] = Math.abs(value);
    }
    return entry;
  });

  // Reduce tick count for X-axis
  const tickInterval = Math.max(0, Math.floor(chartData.length / 15));

  return (
    <div className="p-8">
      <div className="mb-8">
        <h1 className="text-2xl font-bold text-zinc-900 dark:text-zinc-100">Score History</h1>
        <p className="mt-1 text-sm text-zinc-500">
          Default branch health scores over time ({history.length} entries)
        </p>
      </div>

      {/* Score Over Time */}
      <Card className="mb-8">
        <CardHeader>
          <CardTitle>Health Score Trend</CardTitle>
        </CardHeader>
        <CardContent>
          <ResponsiveContainer width="100%" height={300}>
            <LineChart data={chartData}>
              <CartesianGrid strokeDasharray="3 3" stroke="#333" />
              <XAxis
                dataKey="label"
                tick={{ fontSize: 10, fill: "#a1a1aa" }}
                interval={tickInterval}
                angle={allSameDate ? -45 : 0}
                textAnchor={allSameDate ? "end" : "middle"}
                height={allSameDate ? 50 : 30}
              />
              <YAxis
                domain={[0, scoreYMax]}
                tick={{ fontSize: 11, fill: "#a1a1aa" }}
              />
              <Tooltip
                contentStyle={{
                  backgroundColor: "#18181b",
                  border: "1px solid #3f3f46",
                  borderRadius: "8px",
                  fontSize: "12px",
                  color: "#e4e4e7",
                }}
                labelFormatter={(_, payload) => {
                  const item = payload?.[0]?.payload;
                  return item?.commit_sha
                    ? `Commit: ${item.commit_sha.slice(0, 8)} (${item.date})`
                    : item?.date ?? "";
                }}
                formatter={(value) => [String(value ?? 0), "Score"]}
              />
              <ReferenceLine y={0} stroke="#3f3f46" />
              <Line
                type="monotone"
                dataKey="total_score"
                stroke="#10b981"
                strokeWidth={2}
                dot={{ r: 2, fill: "#10b981" }}
                activeDot={{ r: 4 }}
              />
            </LineChart>
          </ResponsiveContainer>
        </CardContent>
      </Card>

      {/* Metric Breakdown Over Time */}
      <Card className="mb-8">
        <CardHeader>
          <CardTitle>Metric Deductions Over Time</CardTitle>
        </CardHeader>
        <CardContent>
          <ResponsiveContainer width="100%" height={300}>
            <AreaChart data={metricData}>
              <CartesianGrid strokeDasharray="3 3" stroke="#333" />
              <XAxis
                dataKey="label"
                tick={{ fontSize: 10, fill: "#a1a1aa" }}
                interval={tickInterval}
                angle={allSameDate ? -45 : 0}
                textAnchor={allSameDate ? "end" : "middle"}
                height={allSameDate ? 50 : 30}
              />
              <YAxis
                domain={[0, metricYMax]}
                tick={{ fontSize: 11, fill: "#a1a1aa" }}
              />
              <Tooltip
                contentStyle={{
                  backgroundColor: "#18181b",
                  border: "1px solid #3f3f46",
                  borderRadius: "8px",
                  fontSize: "12px",
                  color: "#e4e4e7",
                }}
              />
              <Legend
                formatter={(value: string) => METRIC_NAMES[value] ?? value}
              />
              {Object.entries(METRIC_COLORS).map(([key, color]) => (
                <Area
                  key={key}
                  type="monotone"
                  dataKey={key}
                  stackId="1"
                  stroke={color}
                  fill={color}
                  fillOpacity={0.4}
                  name={key}
                />
              ))}
            </AreaChart>
          </ResponsiveContainer>
        </CardContent>
      </Card>

      {/* History Table */}
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <CardTitle>Score History</CardTitle>
            <div className="flex items-center gap-2">
              {/* Grade Filter */}
              <div className="flex items-center gap-1 rounded-lg border border-zinc-200 dark:border-zinc-700 p-0.5">
                <button
                  onClick={() => setGradeFilter(null)}
                  className={`px-2.5 py-1 text-xs font-medium rounded transition-colors ${
                    gradeFilter === null
                      ? "bg-zinc-200 text-zinc-900 dark:bg-zinc-700 dark:text-zinc-100"
                      : "text-zinc-500 hover:text-zinc-700 dark:hover:text-zinc-300"
                  }`}
                >
                  All
                </button>
                {availableGrades.map((g) => (
                  <button
                    key={g}
                    onClick={() => setGradeFilter(gradeFilter === g ? null : g)}
                    className={`px-2 py-1 transition-colors rounded ${
                      gradeFilter === g
                        ? "bg-zinc-200 dark:bg-zinc-700"
                        : "hover:bg-zinc-100 dark:hover:bg-zinc-800"
                    }`}
                  >
                    <GradeBadge grade={g} size="sm" />
                  </button>
                ))}
              </div>
              <span className="text-xs text-zinc-400">
                {filtered.length} entries
              </span>
            </div>
          </div>
        </CardHeader>
        <CardContent>
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-zinc-200 dark:border-zinc-800">
                  <th className="py-2 pr-4 text-left text-xs font-medium text-zinc-500">Date</th>
                  <th className="py-2 pr-4 text-left text-xs font-medium text-zinc-500">Commit</th>
                  <th className="py-2 pr-4 text-left text-xs font-medium text-zinc-500">Grade</th>
                  <th className="py-2 pr-4 text-right text-xs font-medium text-zinc-500">Score</th>
                  {Object.keys(METRIC_NAMES).map((key) => (
                    <th
                      key={key}
                      className="py-2 pr-4 text-right text-xs font-medium text-zinc-500"
                    >
                      {METRIC_NAMES[key]}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {pageData.map((h, i) => (
                  <tr
                    key={`${h.date}-${h.commit_sha}-${i}`}
                    className="border-b border-zinc-100 dark:border-zinc-800/50"
                  >
                    <td className="py-2 pr-4 text-zinc-700 dark:text-zinc-300">
                      {h.date}
                    </td>
                    <td className="py-2 pr-4">
                      {h.commit_sha ? (
                        <a
                          href={`${GITHUB_BASE}/${h.commit_sha}`}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="inline-flex items-center gap-1 font-mono text-xs text-emerald-600 hover:text-emerald-500 hover:underline dark:text-emerald-400"
                        >
                          {h.commit_sha.slice(0, 8)}
                          <ExternalLink className="h-3 w-3" />
                        </a>
                      ) : (
                        <span className="font-mono text-xs text-zinc-400">-</span>
                      )}
                    </td>
                    <td className="py-2 pr-4">
                      <GradeBadge grade={h.grade} size="sm" />
                    </td>
                    <td className="py-2 pr-4 text-right font-mono font-semibold text-zinc-900 dark:text-zinc-100">
                      {h.total_score}
                    </td>
                    {Object.keys(METRIC_NAMES).map((key) => (
                      <td
                        key={key}
                        className={`py-2 pr-4 text-right font-mono text-xs ${
                          (h.metrics[key] ?? 0) < 0
                            ? "text-red-500"
                            : "text-zinc-400"
                        }`}
                      >
                        {h.metrics[key] ?? 0}
                      </td>
                    ))}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {/* Pagination */}
          {totalPages > 1 && (
            <div className="mt-4 flex items-center justify-between border-t border-zinc-200 pt-4 dark:border-zinc-800">
              <span className="text-xs text-zinc-500">
                Showing {page * PAGE_SIZE + 1}–{Math.min((page + 1) * PAGE_SIZE, filtered.length)} of {filtered.length}
              </span>
              <div className="flex items-center gap-2">
                <button
                  onClick={() => setPage((p) => Math.max(0, p - 1))}
                  disabled={page === 0}
                  className="inline-flex items-center gap-1 rounded-lg border border-zinc-200 px-3 py-1.5 text-xs font-medium text-zinc-700 hover:bg-zinc-50 disabled:opacity-40 disabled:cursor-not-allowed dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
                >
                  <ChevronLeft className="h-3 w-3" />
                  Prev
                </button>
                <span className="text-xs text-zinc-500">
                  {page + 1} / {totalPages}
                </span>
                <button
                  onClick={() => setPage((p) => Math.min(totalPages - 1, p + 1))}
                  disabled={page >= totalPages - 1}
                  className="inline-flex items-center gap-1 rounded-lg border border-zinc-200 px-3 py-1.5 text-xs font-medium text-zinc-700 hover:bg-zinc-50 disabled:opacity-40 disabled:cursor-not-allowed dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
                >
                  Next
                  <ChevronRight className="h-3 w-3" />
                </button>
              </div>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
