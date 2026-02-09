import { render, screen, cleanup } from "@testing-library/react";
import { afterEach, describe, it, expect } from "vitest";
import { ScoreBreakdown } from "./score-breakdown";
import type { MetricResult } from "@/lib/types";

afterEach(cleanup);

function makeMetric(overrides: Partial<MetricResult> = {}): MetricResult {
  return {
    key: "test_metric",
    name: "Test Metric",
    contribution: -2.5,
    severity: "MEDIUM",
    evidence: [],
    ...overrides,
  };
}

describe("ScoreBreakdown", () => {
  it("renders the heading", () => {
    render(<ScoreBreakdown metrics={[]} />);
    expect(screen.getByText("Score Breakdown")).toBeInTheDocument();
  });

  it("renders metric names", () => {
    const metrics = [
      makeMetric({ key: "m1", name: "Cyclic Dependencies" }),
      makeMetric({ key: "m2", name: "God Nodes" }),
    ];
    render(<ScoreBreakdown metrics={metrics} />);
    expect(screen.getByText("Cyclic Dependencies")).toBeInTheDocument();
    expect(screen.getByText("God Nodes")).toBeInTheDocument();
  });

  it("displays severity badges", () => {
    const metrics = [makeMetric({ key: "m1", severity: "HIGH" })];
    render(<ScoreBreakdown metrics={metrics} />);
    expect(screen.getByText("HIGH")).toBeInTheDocument();
  });

  it("shows positive contribution with plus sign", () => {
    const metrics = [makeMetric({ key: "m1", contribution: 3.0 })];
    render(<ScoreBreakdown metrics={metrics} />);
    expect(screen.getByText("+3.0")).toBeInTheDocument();
  });

  it("shows negative contribution", () => {
    const metrics = [makeMetric({ key: "m1", contribution: -2.5 })];
    render(<ScoreBreakdown metrics={metrics} />);
    expect(screen.getByText("-2.5")).toBeInTheDocument();
  });

  it("shows zero contribution", () => {
    const metrics = [makeMetric({ key: "m1", contribution: 0 })];
    render(<ScoreBreakdown metrics={metrics} />);
    expect(screen.getByText("0.0")).toBeInTheDocument();
  });

  it("handles metrics with null evidence gracefully", () => {
    const metrics = [
      makeMetric({
        key: "m1",
        name: "No Evidence",
        evidence: undefined as unknown as MetricResult["evidence"],
      }),
    ];
    expect(() => render(<ScoreBreakdown metrics={metrics} />)).not.toThrow();
    expect(screen.getByText("No Evidence")).toBeInTheDocument();
  });

  it("handles metrics with empty evidence array", () => {
    const metrics = [makeMetric({ key: "m1", name: "Empty Evidence", evidence: [] })];
    render(<ScoreBreakdown metrics={metrics} />);
    expect(screen.getByText("Empty Evidence")).toBeInTheDocument();
  });
});
