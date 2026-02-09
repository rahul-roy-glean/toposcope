import { render, screen, cleanup } from "@testing-library/react";
import { afterEach, describe, it, expect } from "vitest";
import { GradeBadge } from "./grade-badge";

afterEach(cleanup);

describe("GradeBadge", () => {
  it("renders the grade letter", () => {
    render(<GradeBadge grade="A" />);
    expect(screen.getByText("A")).toBeInTheDocument();
  });

  it.each(["A", "B", "C", "D", "F"] as const)(
    "applies the correct color class for grade %s",
    (grade) => {
      const { container } = render(<GradeBadge grade={grade} />);
      const el = container.firstElementChild!;

      const expectedColors: Record<string, string> = {
        A: "bg-emerald-500",
        B: "bg-emerald-400",
        C: "bg-amber-400",
        D: "bg-orange-500",
        F: "bg-red-500",
      };

      expect(el.className).toContain(expectedColors[grade]);
    }
  );

  it("applies fallback color for unknown grades", () => {
    const { container } = render(<GradeBadge grade="X" />);
    expect(container.firstElementChild!.className).toContain("bg-zinc-400");
  });

  it("renders with small size", () => {
    const { container } = render(<GradeBadge grade="A" size="sm" />);
    expect(container.firstElementChild!.className).toContain("h-8");
    expect(container.firstElementChild!.className).toContain("w-8");
  });

  it("renders with large size", () => {
    const { container } = render(<GradeBadge grade="A" size="lg" />);
    expect(container.firstElementChild!.className).toContain("h-20");
    expect(container.firstElementChild!.className).toContain("w-20");
  });

  it("defaults to medium size", () => {
    const { container } = render(<GradeBadge grade="B" />);
    expect(container.firstElementChild!.className).toContain("h-12");
    expect(container.firstElementChild!.className).toContain("w-12");
  });

  it("applies custom className", () => {
    const { container } = render(<GradeBadge grade="A" className="my-custom" />);
    expect(container.firstElementChild!.className).toContain("my-custom");
  });
});
