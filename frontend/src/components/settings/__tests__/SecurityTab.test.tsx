import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import "@testing-library/jest-dom";
import { SecurityTab } from "@/components/settings/SecurityTab";

const getCurrentUsername = vi.fn();
const hasMasterPassword = vi.fn();

// Mock the Wails bindings SecurityTab dynamically imports. SecurityTab never
// crashes if these reject, so the assertions below hold against both resolved
// and rejected bindings.
vi.mock("../../../../wailsjs/go/services/SettingsService", () => ({
  GetCurrentUsername: (...args: unknown[]) => getCurrentUsername(...args),
  HasMasterPassword: (...args: unknown[]) => hasMasterPassword(...args),
}));

beforeEach(() => {
  vi.clearAllMocks();
});

describe("SecurityTab", () => {
  it("shows the Registered Accountable Entity (RAE) label with the resolved username", async () => {
    getCurrentUsername.mockResolvedValue("sblanken");
    hasMasterPassword.mockResolvedValue(false);

    render(<SecurityTab />);

    const rae = await screen.findByText(/Registered Accountable Entity \(RAE\):/);
    expect(rae).toBeInTheDocument();
    expect(rae).toHaveTextContent("sblanken");
  });

  it("renders the RAE block before the Master password block", async () => {
    getCurrentUsername.mockResolvedValue("sblanken");
    hasMasterPassword.mockResolvedValue(false);

    render(<SecurityTab />);

    const rae = await screen.findByText(/Registered Accountable Entity \(RAE\):/);
    const master = screen.getByText("Master password");

    // DOCUMENT_POSITION_PRECEDING (2) from "master" toward "rae" means the RAE
    // block precedes the Master password block in document order.
    expect(master.compareDocumentPosition(rae)).toBe(Node.DOCUMENT_POSITION_PRECEDING);
  });

  it("degrades to an unknown caption without blanking the tab when the binding errors", async () => {
    getCurrentUsername.mockRejectedValue("binding unavailable");
    hasMasterPassword.mockResolvedValue(false);

    render(<SecurityTab />);

    const rae = await screen.findByText(/Registered Accountable Entity \(RAE\):/);
    expect(rae).toHaveTextContent("unknown");
    // The rest of the tab still renders.
    expect(screen.getByText("Master password")).toBeInTheDocument();
  });
});