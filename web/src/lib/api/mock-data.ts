import type {
  Repository,
  ScoreResult,
  Snapshot,
  Node,
  Edge,
  ScoreHistory,
} from "@/lib/types";

export const mockRepos: Repository[] = [
  { id: "repo-1", full_name: "acme/monorepo", default_branch: "main" },
  { id: "repo-2", full_name: "acme/platform", default_branch: "main" },
  { id: "repo-3", full_name: "acme/data-pipeline", default_branch: "develop" },
];

function makeNodes(): Record<string, Node> {
  const packages = [
    "//app/auth",
    "//app/gateway",
    "//app/users",
    "//lib/common",
    "//lib/db",
    "//lib/cache",
    "//lib/logging",
    "//svc/notifications",
    "//svc/billing",
    "//tools/codegen",
  ];

  const nodes: Record<string, Node> = {};
  const kinds = ["go_library", "go_binary", "go_test", "proto_library"];

  for (const pkg of packages) {
    const libKey = `${pkg}:lib`;
    nodes[libKey] = {
      key: libKey,
      kind: "go_library",
      package: pkg,
      tags: [],
      visibility: ["//visibility:public"],
      is_test: false,
      is_external: false,
    };

    const binKey = `${pkg}:bin`;
    if (pkg.startsWith("//app") || pkg.startsWith("//svc")) {
      nodes[binKey] = {
        key: binKey,
        kind: "go_binary",
        package: pkg,
        tags: ["deployable"],
        visibility: ["//visibility:public"],
        is_test: false,
        is_external: false,
      };
    }

    const testKey = `${pkg}:test`;
    nodes[testKey] = {
      key: testKey,
      kind: "go_test",
      package: pkg,
      tags: [],
      visibility: ["//visibility:private"],
      is_test: true,
      is_external: false,
    };

    if (pkg.startsWith("//lib")) {
      const protoKey = `${pkg}:proto`;
      nodes[protoKey] = {
        key: protoKey,
        kind: kinds[3],
        package: pkg,
        tags: [],
        visibility: ["//visibility:public"],
        is_test: false,
        is_external: false,
      };
    }
  }

  // External deps
  const externals = [
    "@com_github_gorilla_mux//:mux",
    "@com_github_lib_pq//:pq",
    "@com_github_redis_go_redis//:redis",
  ];
  for (const ext of externals) {
    nodes[ext] = {
      key: ext,
      kind: "go_library",
      package: ext.split("//:")[0],
      tags: [],
      visibility: ["//visibility:public"],
      is_test: false,
      is_external: true,
    };
  }

  return nodes;
}

function makeEdges(nodes: Record<string, Node>): Edge[] {
  const edges: Edge[] = [
    // app/auth dependencies
    { from: "//app/auth:lib", to: "//lib/common:lib", type: "COMPILE" },
    { from: "//app/auth:lib", to: "//lib/db:lib", type: "COMPILE" },
    { from: "//app/auth:lib", to: "//lib/cache:lib", type: "COMPILE" },
    { from: "//app/auth:lib", to: "//lib/logging:lib", type: "COMPILE" },
    { from: "//app/auth:bin", to: "//app/auth:lib", type: "COMPILE" },
    { from: "//app/auth:test", to: "//app/auth:lib", type: "COMPILE" },

    // app/gateway dependencies
    { from: "//app/gateway:lib", to: "//lib/common:lib", type: "COMPILE" },
    { from: "//app/gateway:lib", to: "//app/auth:lib", type: "COMPILE" },
    { from: "//app/gateway:lib", to: "//app/users:lib", type: "COMPILE" },
    { from: "//app/gateway:lib", to: "//lib/logging:lib", type: "COMPILE" },
    { from: "//app/gateway:lib", to: "@com_github_gorilla_mux//:mux", type: "COMPILE" },
    { from: "//app/gateway:bin", to: "//app/gateway:lib", type: "COMPILE" },
    { from: "//app/gateway:test", to: "//app/gateway:lib", type: "COMPILE" },

    // app/users dependencies
    { from: "//app/users:lib", to: "//lib/common:lib", type: "COMPILE" },
    { from: "//app/users:lib", to: "//lib/db:lib", type: "COMPILE" },
    { from: "//app/users:lib", to: "//lib/logging:lib", type: "COMPILE" },
    { from: "//app/users:bin", to: "//app/users:lib", type: "COMPILE" },
    { from: "//app/users:test", to: "//app/users:lib", type: "COMPILE" },

    // lib dependencies
    { from: "//lib/db:lib", to: "//lib/common:lib", type: "COMPILE" },
    { from: "//lib/db:lib", to: "@com_github_lib_pq//:pq", type: "COMPILE" },
    { from: "//lib/db:test", to: "//lib/db:lib", type: "COMPILE" },
    { from: "//lib/cache:lib", to: "//lib/common:lib", type: "COMPILE" },
    { from: "//lib/cache:lib", to: "@com_github_redis_go_redis//:redis", type: "COMPILE" },
    { from: "//lib/cache:test", to: "//lib/cache:lib", type: "COMPILE" },
    { from: "//lib/logging:lib", to: "//lib/common:lib", type: "COMPILE" },
    { from: "//lib/logging:test", to: "//lib/logging:lib", type: "COMPILE" },
    { from: "//lib/common:test", to: "//lib/common:lib", type: "COMPILE" },

    // svc dependencies
    { from: "//svc/notifications:lib", to: "//lib/common:lib", type: "COMPILE" },
    { from: "//svc/notifications:lib", to: "//lib/db:lib", type: "COMPILE" },
    { from: "//svc/notifications:lib", to: "//lib/logging:lib", type: "COMPILE" },
    { from: "//svc/notifications:lib", to: "//app/users:lib", type: "COMPILE" },
    { from: "//svc/notifications:bin", to: "//svc/notifications:lib", type: "COMPILE" },
    { from: "//svc/notifications:test", to: "//svc/notifications:lib", type: "COMPILE" },

    { from: "//svc/billing:lib", to: "//lib/common:lib", type: "COMPILE" },
    { from: "//svc/billing:lib", to: "//lib/db:lib", type: "COMPILE" },
    { from: "//svc/billing:lib", to: "//lib/logging:lib", type: "COMPILE" },
    { from: "//svc/billing:lib", to: "//app/users:lib", type: "COMPILE" },
    { from: "//svc/billing:bin", to: "//svc/billing:lib", type: "COMPILE" },
    { from: "//svc/billing:test", to: "//svc/billing:lib", type: "COMPILE" },

    // tools
    { from: "//tools/codegen:lib", to: "//lib/common:lib", type: "COMPILE" },
    { from: "//tools/codegen:test", to: "//tools/codegen:lib", type: "COMPILE" },

    // Runtime/data edges
    { from: "//app/gateway:lib", to: "//svc/notifications:lib", type: "RUNTIME" },
    { from: "//app/gateway:lib", to: "//svc/billing:lib", type: "RUNTIME" },
    { from: "//lib/db:lib", to: "//lib/common:proto", type: "DATA" },
    { from: "//lib/cache:lib", to: "//lib/common:proto", type: "DATA" },
  ];

  return edges.filter((e) => e.from in nodes && e.to in nodes);
}

const snapshotNodes = makeNodes();
const snapshotEdges = makeEdges(snapshotNodes);

export const mockSnapshot: Snapshot = {
  id: "snap-001",
  commit_sha: "a1b2c3d4e5f6",
  branch: "main",
  partial: false,
  nodes: snapshotNodes,
  edges: snapshotEdges,
  stats: {
    node_count: Object.keys(snapshotNodes).length,
    edge_count: snapshotEdges.length,
    package_count: 10,
    extraction_ms: 342,
  },
  extracted_at: "2025-01-15T10:30:00Z",
};

export const mockScores: Record<string, ScoreResult[]> = {
  "repo-1": [
    {
      total_score: 82,
      grade: "B",
      breakdown: [
        {
          key: "m1_fan_in",
          name: "Fan-In Concentration",
          contribution: -3,
          severity: "LOW",
          evidence: [
            {
              type: "high_fan_in",
              summary: "//lib/common:lib has 12 direct dependents (threshold: 10)",
              to: "//lib/common:lib",
              value: 12,
            },
          ],
        },
        {
          key: "m2_fan_out",
          name: "Fan-Out Breadth",
          contribution: -5,
          severity: "MEDIUM",
          evidence: [
            {
              type: "high_fan_out",
              summary: "//app/gateway:lib depends on 7 targets directly",
              from: "//app/gateway:lib",
              value: 7,
            },
          ],
        },
        {
          key: "m3_dep_depth",
          name: "Dependency Depth",
          contribution: -2,
          severity: "LOW",
          evidence: [
            {
              type: "deep_chain",
              summary: "Longest dependency chain is 5 hops (//app/gateway -> //app/auth -> //lib/db -> //lib/common -> external)",
              value: 5,
            },
          ],
        },
        {
          key: "m4_visibility",
          name: "Visibility Hygiene",
          contribution: -4,
          severity: "MEDIUM",
          evidence: [
            {
              type: "overly_public",
              summary: "3 targets are public but only used within their package",
              value: 3,
            },
          ],
        },
        {
          key: "m5_cycle",
          name: "Cycle Detection",
          contribution: 0,
          severity: "INFO",
          evidence: [],
        },
        {
          key: "m6_churn",
          name: "Churn Amplification",
          contribution: -4,
          severity: "MEDIUM",
          evidence: [
            {
              type: "high_churn",
              summary: "Change in //lib/common:lib impacts 18 downstream targets",
              from: "//lib/common:lib",
              value: 18,
            },
          ],
        },
      ],
      hotspots: [
        {
          node_key: "//lib/common:lib",
          reason: "High fan-in hub with 12 dependents, amplifies churn across 18 targets",
          score_contribution: -7,
          metric_keys: ["m1_fan_in", "m6_churn"],
        },
        {
          node_key: "//app/gateway:lib",
          reason: "High fan-out with 7 direct deps, acts as structural bottleneck",
          score_contribution: -5,
          metric_keys: ["m2_fan_out"],
        },
      ],
      suggested_actions: [
        {
          title: "Split //lib/common into focused modules",
          description: "The common library has become a catch-all dependency. Extract auth utilities, string helpers, and proto definitions into separate packages to reduce fan-in concentration.",
          targets: ["//lib/common:lib"],
          confidence: 0.85,
          addresses: ["m1_fan_in", "m6_churn"],
        },
        {
          title: "Introduce facade for //app/gateway",
          description: "The gateway directly depends on too many internal services. Consider using an interface/facade pattern to reduce direct coupling.",
          targets: ["//app/gateway:lib"],
          confidence: 0.72,
          addresses: ["m2_fan_out"],
        },
        {
          title: "Tighten visibility on 3 over-exposed targets",
          description: "Restrict visibility to package-level for targets that are only used internally.",
          targets: ["//lib/db:proto", "//lib/cache:proto", "//lib/common:proto"],
          confidence: 0.9,
          addresses: ["m4_visibility"],
        },
      ],
      delta_stats: {
        impacted_targets: 8,
        added_nodes: 1,
        removed_nodes: 0,
        added_edges: 3,
        removed_edges: 1,
      },
      commit_sha: "def5678abc1234def5678abc1234def5678abc12",
      base_snapshot_id: "snap-base-1",
      head_snapshot_id: "snap-head-1",
      delta_id: "delta-1",
      pr_number: 142,
      created_at: "2025-01-15T10:35:00Z",
    },
    {
      total_score: 65,
      grade: "C",
      breakdown: [
        {
          key: "m1_fan_in",
          name: "Fan-In Concentration",
          contribution: -8,
          severity: "HIGH",
          evidence: [
            {
              type: "high_fan_in",
              summary: "//lib/common:lib has 15 direct dependents (threshold: 10)",
              to: "//lib/common:lib",
              value: 15,
            },
          ],
        },
        {
          key: "m2_fan_out",
          name: "Fan-Out Breadth",
          contribution: -5,
          severity: "MEDIUM",
          evidence: [
            {
              type: "high_fan_out",
              summary: "//app/gateway:lib depends on 9 targets directly",
              from: "//app/gateway:lib",
              value: 9,
            },
          ],
        },
        {
          key: "m3_dep_depth",
          name: "Dependency Depth",
          contribution: -3,
          severity: "LOW",
          evidence: [
            {
              type: "deep_chain",
              summary: "Longest dependency chain is 6 hops",
              value: 6,
            },
          ],
        },
        {
          key: "m4_visibility",
          name: "Visibility Hygiene",
          contribution: -6,
          severity: "MEDIUM",
          evidence: [
            {
              type: "overly_public",
              summary: "5 targets are public but only used within their package",
              value: 5,
            },
          ],
        },
        {
          key: "m5_cycle",
          name: "Cycle Detection",
          contribution: -5,
          severity: "HIGH",
          evidence: [
            {
              type: "cycle",
              summary: "Cycle detected: //app/auth:lib -> //app/users:lib -> //app/auth:lib",
              from: "//app/auth:lib",
              to: "//app/users:lib",
            },
          ],
        },
        {
          key: "m6_churn",
          name: "Churn Amplification",
          contribution: -8,
          severity: "HIGH",
          evidence: [
            {
              type: "high_churn",
              summary: "Change in //lib/common:lib impacts 24 downstream targets",
              from: "//lib/common:lib",
              value: 24,
            },
          ],
        },
      ],
      hotspots: [
        {
          node_key: "//lib/common:lib",
          reason: "Extreme fan-in hub with 15 dependents, amplifies churn across 24 targets",
          score_contribution: -16,
          metric_keys: ["m1_fan_in", "m6_churn"],
        },
        {
          node_key: "//app/auth:lib",
          reason: "Part of dependency cycle with //app/users:lib",
          score_contribution: -5,
          metric_keys: ["m5_cycle"],
        },
      ],
      suggested_actions: [
        {
          title: "Break cycle between auth and users",
          description: "Extract shared types into a new //lib/identity package to eliminate the circular dependency.",
          targets: ["//app/auth:lib", "//app/users:lib"],
          confidence: 0.92,
          addresses: ["m5_cycle"],
        },
      ],
      delta_stats: {
        impacted_targets: 14,
        added_nodes: 3,
        removed_nodes: 0,
        added_edges: 5,
        removed_edges: 0,
      },
      commit_sha: "222bbbb111aaaa222bbbb111aaaa222bbbb111aaa",
      base_snapshot_id: "snap-base-2",
      head_snapshot_id: "snap-head-2",
      delta_id: "delta-2",
      pr_number: 138,
      created_at: "2025-01-14T16:20:00Z",
    },
    {
      total_score: 91,
      grade: "A",
      breakdown: [
        {
          key: "m1_fan_in",
          name: "Fan-In Concentration",
          contribution: -2,
          severity: "LOW",
          evidence: [],
        },
        {
          key: "m2_fan_out",
          name: "Fan-Out Breadth",
          contribution: -1,
          severity: "INFO",
          evidence: [],
        },
        {
          key: "m3_dep_depth",
          name: "Dependency Depth",
          contribution: -1,
          severity: "INFO",
          evidence: [],
        },
        {
          key: "m4_visibility",
          name: "Visibility Hygiene",
          contribution: -2,
          severity: "LOW",
          evidence: [],
        },
        {
          key: "m5_cycle",
          name: "Cycle Detection",
          contribution: 0,
          severity: "INFO",
          evidence: [],
        },
        {
          key: "m6_churn",
          name: "Churn Amplification",
          contribution: -3,
          severity: "LOW",
          evidence: [
            {
              type: "high_churn",
              summary: "Change impacts 8 downstream targets",
              value: 8,
            },
          ],
        },
      ],
      hotspots: [],
      suggested_actions: [],
      delta_stats: {
        impacted_targets: 3,
        added_nodes: 0,
        removed_nodes: 1,
        added_edges: 0,
        removed_edges: 2,
      },
      commit_sha: "444dddd333cccc444dddd333cccc444dddd333ccc",
      base_snapshot_id: "snap-base-3",
      head_snapshot_id: "snap-head-3",
      delta_id: "delta-3",
      pr_number: 145,
      created_at: "2025-01-16T09:15:00Z",
    },
  ],
  "repo-2": [
    {
      total_score: 74,
      grade: "C",
      breakdown: [
        {
          key: "m1_fan_in",
          name: "Fan-In Concentration",
          contribution: -6,
          severity: "MEDIUM",
          evidence: [],
        },
        {
          key: "m2_fan_out",
          name: "Fan-Out Breadth",
          contribution: -4,
          severity: "MEDIUM",
          evidence: [],
        },
        {
          key: "m3_dep_depth",
          name: "Dependency Depth",
          contribution: -3,
          severity: "LOW",
          evidence: [],
        },
        {
          key: "m4_visibility",
          name: "Visibility Hygiene",
          contribution: -5,
          severity: "MEDIUM",
          evidence: [],
        },
        {
          key: "m5_cycle",
          name: "Cycle Detection",
          contribution: 0,
          severity: "INFO",
          evidence: [],
        },
        {
          key: "m6_churn",
          name: "Churn Amplification",
          contribution: -8,
          severity: "HIGH",
          evidence: [],
        },
      ],
      hotspots: [
        {
          node_key: "//platform/core:lib",
          reason: "Central hub with excessive churn amplification",
          score_contribution: -12,
          metric_keys: ["m1_fan_in", "m6_churn"],
        },
      ],
      suggested_actions: [],
      delta_stats: {
        impacted_targets: 11,
        added_nodes: 2,
        removed_nodes: 1,
        added_edges: 4,
        removed_edges: 2,
      },
      commit_sha: "bbb222aaa111bbb222aaa111bbb222aaa111bbb22",
      base_snapshot_id: "snap-base-4",
      head_snapshot_id: "snap-head-4",
      delta_id: "delta-4",
      pr_number: 87,
      created_at: "2025-01-15T14:00:00Z",
    },
  ],
  "repo-3": [
    {
      total_score: 55,
      grade: "D",
      breakdown: [
        {
          key: "m1_fan_in",
          name: "Fan-In Concentration",
          contribution: -10,
          severity: "HIGH",
          evidence: [],
        },
        {
          key: "m2_fan_out",
          name: "Fan-Out Breadth",
          contribution: -8,
          severity: "HIGH",
          evidence: [],
        },
        {
          key: "m3_dep_depth",
          name: "Dependency Depth",
          contribution: -5,
          severity: "MEDIUM",
          evidence: [],
        },
        {
          key: "m4_visibility",
          name: "Visibility Hygiene",
          contribution: -7,
          severity: "MEDIUM",
          evidence: [],
        },
        {
          key: "m5_cycle",
          name: "Cycle Detection",
          contribution: -8,
          severity: "HIGH",
          evidence: [],
        },
        {
          key: "m6_churn",
          name: "Churn Amplification",
          contribution: -7,
          severity: "MEDIUM",
          evidence: [],
        },
      ],
      hotspots: [
        {
          node_key: "//pipeline/etl:lib",
          reason: "Multiple structural issues: cycles, high fan-out, deep chains",
          score_contribution: -18,
          metric_keys: ["m2_fan_out", "m5_cycle", "m3_dep_depth"],
        },
      ],
      suggested_actions: [
        {
          title: "Decompose monolithic ETL pipeline",
          description: "Split the single ETL target into stage-specific packages (extract, transform, load) to reduce coupling.",
          targets: ["//pipeline/etl:lib"],
          confidence: 0.88,
          addresses: ["m2_fan_out", "m5_cycle"],
        },
      ],
      delta_stats: {
        impacted_targets: 22,
        added_nodes: 5,
        removed_nodes: 0,
        added_edges: 8,
        removed_edges: 1,
      },
      commit_sha: "yyy222xxx111yyy222xxx111yyy222xxx111yyy22",
      base_snapshot_id: "snap-base-5",
      head_snapshot_id: "snap-head-5",
      delta_id: "delta-5",
      pr_number: 34,
      created_at: "2025-01-13T11:00:00Z",
    },
  ],
};

export const mockScoreHistory: Record<string, ScoreHistory[]> = {
  "repo-1": Array.from({ length: 20 }, (_, i) => {
    const date = new Date(2025, 0, 1 + i);
    const baseScore = 78 + Math.sin(i * 0.5) * 8 + (i > 15 ? 5 : 0);
    const score = Math.round(Math.min(100, Math.max(0, baseScore)));
    return {
      date: date.toISOString().split("T")[0],
      commit_sha: `abc${i.toString().padStart(4, "0")}def0123456789abcdef0123456789abcdef`,
      total_score: score,
      grade: score >= 90 ? "A" : score >= 80 ? "B" : score >= 70 ? "C" : score >= 60 ? "D" : "F",
      metrics: {
        m1_fan_in: Math.round(-3 + Math.sin(i * 0.3) * 2),
        m2_fan_out: Math.round(-4 + Math.cos(i * 0.4) * 2),
        m3_dep_depth: Math.round(-2 + Math.sin(i * 0.2)),
        m4_visibility: Math.round(-4 + Math.cos(i * 0.5) * 2),
        m5_cycle: i > 10 ? 0 : Math.round(-3 + Math.sin(i * 0.6) * 2),
        m6_churn: Math.round(-5 + Math.sin(i * 0.35) * 3),
      },
    };
  }),
  "repo-2": Array.from({ length: 20 }, (_, i) => {
    const date = new Date(2025, 0, 1 + i);
    const score = Math.round(Math.min(100, Math.max(0, 70 + Math.sin(i * 0.4) * 6)));
    return {
      date: date.toISOString().split("T")[0],
      commit_sha: `bcd${i.toString().padStart(4, "0")}ef0123456789abcdef0123456789abcdef0`,
      total_score: score,
      grade: score >= 90 ? "A" : score >= 80 ? "B" : score >= 70 ? "C" : score >= 60 ? "D" : "F",
      metrics: {
        m1_fan_in: Math.round(-5 + Math.sin(i * 0.3) * 3),
        m2_fan_out: Math.round(-4 + Math.cos(i * 0.4) * 2),
        m3_dep_depth: Math.round(-3),
        m4_visibility: Math.round(-5 + Math.sin(i * 0.5) * 2),
        m5_cycle: 0,
        m6_churn: Math.round(-7 + Math.sin(i * 0.35) * 3),
      },
    };
  }),
  "repo-3": Array.from({ length: 20 }, (_, i) => {
    const date = new Date(2025, 0, 1 + i);
    const score = Math.round(Math.min(100, Math.max(0, 55 + Math.sin(i * 0.3) * 5 - i * 0.5)));
    return {
      date: date.toISOString().split("T")[0],
      commit_sha: `cde${i.toString().padStart(4, "0")}f0123456789abcdef0123456789abcdef01`,
      total_score: score,
      grade: score >= 90 ? "A" : score >= 80 ? "B" : score >= 70 ? "C" : score >= 60 ? "D" : "F",
      metrics: {
        m1_fan_in: Math.round(-8 + Math.sin(i * 0.3) * 2),
        m2_fan_out: Math.round(-7 + Math.cos(i * 0.4) * 2),
        m3_dep_depth: Math.round(-5),
        m4_visibility: Math.round(-6 + Math.sin(i * 0.5)),
        m5_cycle: Math.round(-6 + Math.sin(i * 0.6) * 2),
        m6_churn: Math.round(-6 + Math.sin(i * 0.35) * 2),
      },
    };
  }),
};
