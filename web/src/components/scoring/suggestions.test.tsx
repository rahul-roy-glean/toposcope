import { render, screen, cleanup, within } from "@testing-library/react";
import { afterEach, describe, it, expect } from "vitest";
import { Suggestions } from "./suggestions";
import type { SuggestedAction } from "@/lib/types";

afterEach(cleanup);

function makeAction(overrides: Partial<SuggestedAction> = {}): SuggestedAction {
  return {
    title: "Split the monolith",
    description: "Break large package into smaller modules",
    targets: ["//pkg:core"],
    confidence: 0.85,
    addresses: ["cyclic_deps"],
    ...overrides,
  };
}

describe("Suggestions", () => {
  it("renders empty state when actions is empty", () => {
    render(<Suggestions actions={[]} />);
    expect(screen.getByText("No suggested actions")).toBeInTheDocument();
  });

  it("renders empty state when actions is null-ish", () => {
    render(<Suggestions actions={undefined as unknown as SuggestedAction[]} />);
    expect(screen.getByText("No suggested actions")).toBeInTheDocument();
  });

  it("renders action titles", () => {
    const actions = [
      makeAction({ title: "Split package" }),
      makeAction({ title: "Remove cycle" }),
    ];
    render(<Suggestions actions={actions} />);
    expect(screen.getByText("Split package")).toBeInTheDocument();
    expect(screen.getByText("Remove cycle")).toBeInTheDocument();
  });

  it("renders action descriptions", () => {
    const actions = [makeAction({ description: "A detailed description" })];
    render(<Suggestions actions={actions} />);
    expect(screen.getByText("A detailed description")).toBeInTheDocument();
  });

  it("renders confidence as percentage", () => {
    const actions = [makeAction({ confidence: 0.92 })];
    const { container } = render(<Suggestions actions={actions} />);
    const confidenceEl = container.querySelector("span.rounded-full");
    expect(confidenceEl).toHaveTextContent("92% confidence");
  });

  it("renders target nodes", () => {
    const actions = [makeAction({ targets: ["//a:b", "//c:d"] })];
    render(<Suggestions actions={actions} />);
    expect(screen.getByText("//a:b")).toBeInTheDocument();
    expect(screen.getByText("//c:d")).toBeInTheDocument();
  });

  it("renders addresses", () => {
    const actions = [
      makeAction({
        title: "Unique action",
        addresses: ["cyclic_deps", "god_node"],
      }),
    ];
    render(<Suggestions actions={actions} />);
    expect(screen.getByText("cyclic_deps")).toBeInTheDocument();
    expect(screen.getByText("god_node")).toBeInTheDocument();
  });

  it("handles action with empty targets", () => {
    const actions = [makeAction({ title: "Action with no targets", targets: [] })];
    render(<Suggestions actions={actions} />);
    expect(screen.getByText("Action with no targets")).toBeInTheDocument();
  });

  it("handles action with empty addresses", () => {
    const actions = [makeAction({ title: "No addr action", addresses: [] })];
    render(<Suggestions actions={actions} />);
    expect(screen.getByText("No addr action")).toBeInTheDocument();
    expect(screen.queryByText("Addresses:")).not.toBeInTheDocument();
  });

  it("handles null targets and addresses gracefully", () => {
    const actions = [
      makeAction({
        targets: undefined as unknown as string[],
        addresses: undefined as unknown as string[],
      }),
    ];
    expect(() => render(<Suggestions actions={actions} />)).not.toThrow();
  });
});
