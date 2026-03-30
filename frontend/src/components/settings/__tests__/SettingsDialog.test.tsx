import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import "@testing-library/jest-dom";
import userEvent from "@testing-library/user-event";
import { SettingsDialog } from "@/components/settings/SettingsDialog";

// Mock the wailsjs SettingsService
vi.mock("../../../../wailsjs/go/services/SettingsService", () => ({
  GetSettings: vi.fn().mockResolvedValue({}),
  GetAPIKeyStatus: vi.fn().mockResolvedValue(""),
  SaveSettings: vi.fn().mockResolvedValue(undefined),
  SaveAPIKey: vi.fn().mockResolvedValue(undefined),
  TestConnection: vi.fn().mockResolvedValue("Connected"),
  SetModel: vi.fn().mockResolvedValue(""),
}));

// Mock useTheme for AppearanceTab
vi.mock("@/theme/theme-provider", () => ({
  useTheme: () => ({ theme: "dark", setTheme: vi.fn() }),
}));

beforeEach(() => {
  vi.clearAllMocks();
});

describe("SettingsDialog", () => {
  it("renders Dialog.Title 'Settings' when open=true", () => {
    render(<SettingsDialog open={true} onClose={vi.fn()} />);
    expect(screen.getByText("Settings")).toBeInTheDocument();
  });

  it("renders all 5 tab labels when open=true", () => {
    render(<SettingsDialog open={true} onClose={vi.fn()} />);
    expect(screen.getByText("LLM Config")).toBeInTheDocument();
    expect(screen.getByText("Prompts")).toBeInTheDocument();
    expect(screen.getByText("Terminals")).toBeInTheDocument();
    expect(screen.getByText("Hotkeys")).toBeInTheDocument();
    expect(screen.getByText("Appearance")).toBeInTheDocument();
  });

  it("does not render dialog content when open=false", () => {
    render(<SettingsDialog open={false} onClose={vi.fn()} />);
    expect(screen.queryByText("Settings")).not.toBeInTheDocument();
    expect(screen.queryByText("LLM Config")).not.toBeInTheDocument();
  });

  it("clicking Prompts tab switches to prompts panel", async () => {
    const user = userEvent.setup();
    render(<SettingsDialog open={true} onClose={vi.fn()} />);

    const promptsTab = screen.getByText("Prompts");
    await user.click(promptsTab);

    // PromptsTab renders a textarea for custom prompt
    expect(screen.getByPlaceholderText("Add custom instructions to extend the system prompt...")).toBeInTheDocument();
  });

  it("clicking Terminals tab switches to terminals panel", async () => {
    const user = userEvent.setup();
    render(<SettingsDialog open={true} onClose={vi.fn()} />);

    const terminalsTab = screen.getByText("Terminals");
    await user.click(terminalsTab);

    expect(screen.getByText("Capture Settings")).toBeInTheDocument();
  });

  it("clicking Appearance tab shows theme buttons", async () => {
    const user = userEvent.setup();
    render(<SettingsDialog open={true} onClose={vi.fn()} />);

    const appearanceTab = screen.getByText("Appearance");
    await user.click(appearanceTab);

    expect(screen.getByText("Dark")).toBeInTheDocument();
    expect(screen.getByText("Light")).toBeInTheDocument();
  });
});
