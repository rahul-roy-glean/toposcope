export interface Node {
  key: string;
  kind: string;
  package: string;
  tags: string[];
  visibility: string[];
  is_test: boolean;
  is_external: boolean;
}

export type EdgeType = "COMPILE" | "RUNTIME" | "TOOLCHAIN" | "DATA";

export interface Edge {
  from: string;
  to: string;
  type: EdgeType;
}

export interface SnapshotStats {
  node_count: number;
  edge_count: number;
  package_count: number;
  extraction_ms: number;
}

export interface Snapshot {
  id: string;
  commit_sha: string;
  branch: string;
  partial: boolean;
  nodes: Record<string, Node>;
  edges: Edge[];
  stats: SnapshotStats;
  extracted_at: string;
}

export interface DeltaStats {
  impacted_targets: number;
  added_nodes: number;
  removed_nodes: number;
  added_edges: number;
  removed_edges: number;
}

export interface EvidenceItem {
  type: string;
  summary: string;
  from?: string;
  to?: string;
  value?: number;
}

export type Severity = "HIGH" | "MEDIUM" | "LOW" | "INFO";

export interface MetricResult {
  key: string;
  name: string;
  contribution: number;
  severity: Severity;
  evidence: EvidenceItem[];
}

export interface Hotspot {
  node_key: string;
  reason: string;
  score_contribution: number;
  metric_keys: string[];
}

export interface SuggestedAction {
  title: string;
  description: string;
  targets: string[];
  confidence: number;
  addresses: string[];
}

export interface ScoreResult {
  id?: string;
  total_score: number;
  grade: string;
  breakdown: MetricResult[];
  hotspots: Hotspot[];
  suggested_actions: SuggestedAction[];
  delta_stats: DeltaStats;
  base_commit: string;
  head_commit: string;
  pr_number?: number;
  analyzed_at?: string;
}

export interface Repository {
  id: string;
  full_name: string;
  default_branch: string;
}

export interface ScoreHistory {
  date: string;
  commit_sha: string;
  total_score: number;
  grade: string;
  metrics: Record<string, number>;
}

export interface Subgraph {
  nodes: Record<string, Node>;
  edges: Edge[];
}

export interface PackageNode {
  package: string;
  target_count: number;
  kinds: string[];
  has_tests: boolean;
  is_external: boolean;
}

export interface PackageEdge {
  from: string;
  to: string;
  weight: number;
}

export interface PackageGraph {
  nodes: Record<string, PackageNode>;
  edges: PackageEdge[];
  truncated: boolean;
}

export interface EgoGraph {
  nodes: Record<string, Node>;
  edges: Edge[];
  truncated: boolean;
}

export interface PathResult {
  paths: string[][];
  nodes: Record<string, Node>;
  edges: Edge[];
  from: string;
  to: string;
  path_length: number;
}
