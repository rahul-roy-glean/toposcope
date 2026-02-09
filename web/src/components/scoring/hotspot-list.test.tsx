import { render, screen, cleanup } from "@testing-library/react";
import { afterEach, describe, it, expect } from "vitest";
import { HotspotList } from "./hotspot-list";
import type { Hotspot } from "@/lib/types";

afterEach(cleanup);

function makeHotspot(overrides: Partial<Hotspot> = {}): Hotspot {
  return {
    node_key: "//pkg:target",
    reason: "high coupling",
    score_contribution: 5.2,
    metric_keys: ["cyclic_deps"],
    ...overrides,
  };
}

describe("HotspotList", () => {
  it("renders empty state when hotspots is empty", () => {
    render(<HotspotList hotspots={[]} />);
    expect(screen.getByText("No structural hotspots detected")).toBeInTheDocument();
  });

  it("renders empty state when hotspots is null-ish", () => {
    render(<HotspotList hotspots={undefined as unknown as Hotspot[]} />);
    expect(screen.getByText("No structural hotspots detected")).toBeInTheDocument();
  });

  it("renders hotspot node keys", () => {
    const hotspots = [
      makeHotspot({ node_key: "//foo:bar" }),
      makeHotspot({ node_key: "//baz:qux" }),
    ];
    render(<HotspotList hotspots={hotspots} />);
    expect(screen.getByText("//foo:bar")).toBeInTheDocument();
    expect(screen.getByText("//baz:qux")).toBeInTheDocument();
  });

  it("renders score contribution", () => {
    const hotspots = [makeHotspot({ score_contribution: 3.7 })];
    render(<HotspotList hotspots={hotspots} />);
    expect(screen.getByText("+3.7")).toBeInTheDocument();
  });

  it("renders metric keys with underscores replaced by spaces", () => {
    const hotspots = [
      makeHotspot({
        node_key: "//unique:node",
        metric_keys: ["cyclic_deps", "god_node"],
      }),
    ];
    render(<HotspotList hotspots={hotspots} />);
    expect(screen.getByText("cyclic deps")).toBeInTheDocument();
    expect(screen.getByText("god node")).toBeInTheDocument();
  });

  it("handles hotspot with empty metric_keys", () => {
    const hotspots = [makeHotspot({ metric_keys: [] })];
    render(<HotspotList hotspots={hotspots} />);
    expect(screen.getByText("//pkg:target")).toBeInTheDocument();
  });

  it("handles hotspot with null metric_keys", () => {
    const hotspots = [
      makeHotspot({ metric_keys: undefined as unknown as string[] }),
    ];
    expect(() => render(<HotspotList hotspots={hotspots} />)).not.toThrow();
  });
});
