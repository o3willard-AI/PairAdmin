import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import "@testing-library/jest-dom";
import { AboutTab } from "@/components/settings/AboutTab";

const getVersion = vi.fn();
// Same dynamic-import mock pattern as TerminalsTab.test.tsx — the component
// loads SettingsService via import(...), which resolves to this shim.
vi.mock("../../../../wailsjs/go/services/SettingsService", () => ({
  GetVersion: (...args: unknown[]) => getVersion(...args),
}));

describe("AboutTab", () => {
  it("renders the app name and version once GetVersion resolves", async () => {
    getVersion.mockResolvedValue("v2.6.2");
    render(<AboutTab />);

    // Pre-resolution loading state renders without crashing (no flash).
    expect(screen.getByText("PairAdmin")).toBeInTheDocument();
    expect(screen.getByText("…")).toBeInTheDocument();

    // Mutation check: if the component stops calling GetVersion (or renders
    // an empty/hardcoded version), the mock's value never appears and the
    // call count stays 0 — this fails. The version must come from the binding.
    expect(await screen.findByText("v2.6.2")).toBeInTheDocument();
    expect(getVersion).toHaveBeenCalledTimes(1);
  });

  it("shows a graceful fallback when GetVersion rejects", async () => {
    getVersion.mockRejectedValue(new Error("binding unavailable"));
    render(<AboutTab />);

    expect(screen.getByText("PairAdmin")).toBeInTheDocument();
    // Mutation check: if the rejection path threw instead of rendering the
    // fallback text, this find would time out / error — it must not crash.
    expect(await screen.findByText("version unavailable")).toBeInTheDocument();
  });

  it("treats an empty version as unavailable", async () => {
    getVersion.mockResolvedValue("");
    render(<AboutTab />);
    // Mutation check: passing through the empty string (or a blank) would
    // leave the "…" loading state forever instead of the fallback.
    expect(await screen.findByText("version unavailable")).toBeInTheDocument();
  });
});