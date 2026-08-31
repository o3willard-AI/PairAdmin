import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import "@testing-library/jest-dom";
import userEvent from "@testing-library/user-event";
import App from "@/App";
import { ThreeColumnLayout } from "@/components/layout/ThreeColumnLayout";

const needsMasterPassword = vi.fn();
const hasMasterPassword = vi.fn();
const setMasterPassword = vi.fn();
const verifyMasterPassword = vi.fn();
const loadAPIKeys = vi.fn();

// Path depth: this file sits at src/__tests__/, so the wailsjs root is two
// levels up — it must resolve to the SAME module App.tsx imports
// (src/App.tsx -> ../wailsjs/go/services/SettingsService).
vi.mock("../../wailsjs/go/services/SettingsService", () => ({
  NeedsMasterPassword: (...args: unknown[]) => needsMasterPassword(...args),
  HasMasterPassword: (...args: unknown[]) => hasMasterPassword(...args),
  SetMasterPassword: (...args: unknown[]) => setMasterPassword(...args),
  VerifyMasterPassword: (...args: unknown[]) => verifyMasterPassword(...args),
  LoadAPIKeys: (...args: unknown[]) => loadAPIKeys(...args),
}));

// The three-pane layout needs terminal/command stores and DOM APIs that are
// irrelevant to the gate; stub it to a marker so App's gating is what's under
// test.
vi.mock("@/components/layout/ThreeColumnLayout", () => ({
  ThreeColumnLayout: () => <div data-testid="app-layout" />,
}));

beforeEach(() => {
  vi.clearAllMocks();
  loadAPIKeys.mockResolvedValue(undefined);
});

describe("App startup gating", () => {
  it("OS keychain functional: no dialog, LoadAPIKeys called, layout renders", async () => {
    needsMasterPassword.mockResolvedValue(false);
    render(<App />);
    await waitFor(() => expect(loadAPIKeys).toHaveBeenCalledTimes(1));
    expect(screen.getByTestId("app-layout")).toBeInTheDocument();
    expect(screen.queryByText("Set Master Password")).not.toBeInTheDocument();
    expect(screen.queryByText("Unlock PairAdmin")).not.toBeInTheDocument();
    expect(hasMasterPassword).not.toHaveBeenCalled();
  });

  it("file backend, first run: 'set' dialog; LoadAPIKeys runs after SetMasterPassword", async () => {
    const user = userEvent.setup();
    needsMasterPassword.mockResolvedValue(true);
    hasMasterPassword.mockResolvedValue(false);
    setMasterPassword.mockResolvedValue(undefined);
    render(<App />);

    expect(await screen.findByText("Set Master Password")).toBeInTheDocument();
    expect(loadAPIKeys).not.toHaveBeenCalled();

    await user.type(screen.getByLabelText("Master password"), "hunter2");
    await user.type(screen.getByLabelText("Confirm master password"), "hunter2");
    await user.click(screen.getByRole("button", { name: "Create Master Password" }));

    await waitFor(() => {
      expect(setMasterPassword).toHaveBeenCalledWith("hunter2");
      expect(loadAPIKeys).toHaveBeenCalledTimes(1);
    });
    expect(await screen.findByTestId("app-layout")).toBeInTheDocument();
    expect(screen.queryByText("Set Master Password")).not.toBeInTheDocument();
  });

  it("file backend, subsequent launch: 'unlock' dialog; LoadAPIKeys runs after VerifyMasterPassword", async () => {
    const user = userEvent.setup();
    needsMasterPassword.mockResolvedValue(true);
    hasMasterPassword.mockResolvedValue(true);
    verifyMasterPassword.mockResolvedValue(true);
    render(<App />);

    expect(await screen.findByText("Unlock PairAdmin")).toBeInTheDocument();
    expect(loadAPIKeys).not.toHaveBeenCalled();

    await user.type(screen.getByLabelText("Master password"), "hunter2");
    await user.click(screen.getByRole("button", { name: "Unlock" }));

    await waitFor(() => {
      expect(verifyMasterPassword).toHaveBeenCalledWith("hunter2");
      expect(loadAPIKeys).toHaveBeenCalledTimes(1);
    });
    expect(await screen.findByTestId("app-layout")).toBeInTheDocument();
  });

  it("wrong password at unlock keeps the gate up and defers LoadAPIKeys", async () => {
    const user = userEvent.setup();
    needsMasterPassword.mockResolvedValue(true);
    hasMasterPassword.mockResolvedValue(true);
    verifyMasterPassword.mockResolvedValue(false);
    render(<App />);

    expect(await screen.findByText("Unlock PairAdmin")).toBeInTheDocument();
    await user.type(screen.getByLabelText("Master password"), "nope");
    await user.click(screen.getByRole("button", { name: "Unlock" }));

    expect(await screen.findByText("Incorrect master password.")).toBeInTheDocument();
    expect(loadAPIKeys).not.toHaveBeenCalled();
    expect(screen.getByText("Unlock PairAdmin")).toBeInTheDocument();
  });

  it("LoadAPIKeys failure does not block the app", async () => {
    needsMasterPassword.mockResolvedValue(false);
    loadAPIKeys.mockRejectedValue("keychain exploded");
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
    render(<App />);
    expect(await screen.findByTestId("app-layout")).toBeInTheDocument();
    expect(loadAPIKeys).toHaveBeenCalledTimes(1);
    expect(warn).toHaveBeenCalled();
    warn.mockRestore();
  });

  it("gate probe failure (binding unavailable) does not wedge the app", async () => {
    needsMasterPassword.mockRejectedValue(new Error("window.go is undefined"));
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
    render(<App />);
    expect(await screen.findByTestId("app-layout")).toBeInTheDocument();
    expect(loadAPIKeys).not.toHaveBeenCalled();
    expect(warn).toHaveBeenCalled();
    warn.mockRestore();
  });
});
