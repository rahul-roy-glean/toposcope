"use client";

import { useEffect, useState } from "react";
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
} from "recharts";
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

export default function ScoreHistoryPage() {
  const params = useParams<{ repoId: string }>();
  const [history, setHistory] = useState<ScoreHistory[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    async function load() {
      const api = await getAPI();
      const h = await api.getScoreHistory(params.repoId);
      setHistory(h);
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

  // Transform data for the stacked area chart (make contributions absolute)
  const metricData = history.map((h) => {
    const entry: Record<string, number | string> = { date: h.date };
    for (const [key, value] of Object.entries(h.metrics)) {
      entry[key] = Math.abs(value);
    }
    return entry;
  });

  return (
    <div className="p-8">
      <div className="mb-8">
        <h1 className="text-2xl font-bold text-zinc-900 dark:text-zinc-100">Score History</h1>
        <p className="mt-1 text-sm text-zinc-500">
          Health score trends over time
        </p>
      </div>

      {/* Score Over Time */}
      <Card className="mb-8">
        <CardHeader>
          <CardTitle>Health Score Trend</CardTitle>
        </CardHeader>
        <CardContent>
          <ResponsiveContainer width="100%" height={300}>
            <LineChart data={history}>
              <CartesianGrid strokeDasharray="3 3" stroke="#e4e4e7" />
              <XAxis
                dataKey="date"
                tick={{ fontSize: 11, fill: "#a1a1aa" }}
                tickFormatter={(v: string) => v.slice(5)}
              />
              <YAxis
                domain={[0, 100]}
                tick={{ fontSize: 11, fill: "#a1a1aa" }}
              />
              <Tooltip
                contentStyle={{
                  backgroundColor: "#fff",
                  border: "1px solid #e4e4e7",
                  borderRadius: "8px",
                  fontSize: "12px",
                }}
                formatter={(value) => [String(value ?? 0), "Score"]}
              />
              <Line
                type="monotone"
                dataKey="total_score"
                stroke="#10b981"
                strokeWidth={2}
                dot={{ r: 3, fill: "#10b981" }}
                activeDot={{ r: 5 }}
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
              <CartesianGrid strokeDasharray="3 3" stroke="#e4e4e7" />
              <XAxis
                dataKey="date"
                tick={{ fontSize: 11, fill: "#a1a1aa" }}
                tickFormatter={(v: string) => v.slice(5)}
              />
              <YAxis tick={{ fontSize: 11, fill: "#a1a1aa" }} />
              <Tooltip
                contentStyle={{
                  backgroundColor: "#fff",
                  border: "1px solid #e4e4e7",
                  borderRadius: "8px",
                  fontSize: "12px",
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
          <CardTitle>Score History</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-zinc-200 dark:border-zinc-800">
                  <th className="py-2 pr-4 text-left text-xs font-medium text-zinc-500">Date</th>
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
                {[...history].reverse().map((h) => (
                  <tr
                    key={h.date}
                    className="border-b border-zinc-100 dark:border-zinc-800/50"
                  >
                    <td className="py-2 pr-4 text-zinc-700 dark:text-zinc-300">
                      {h.date}
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
        </CardContent>
      </Card>
    </div>
  );
}
