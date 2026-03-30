import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import "@testing-library/jest-dom";
import userEvent from "@testing-library/user-event";
import { ChatPane } from "@/components/chat/ChatPane";
import { useChatStore } from "@/stores/chatStore";
import { useTerminalStore } from "@/stores/terminalStore";

// Mock Wails LLMService
vi.mock("../../../../wailsjs/go/services/LLMService", () => ({
  FilterCommand: vi.fn().mockResolvedValue("Filter updated"),
  SendMessage: vi.fn().mockResolvedValue(undefined),
}));

// Mock Wails SettingsService
vi.mock("../../../../wailsjs/go/services/SettingsService", () => ({
  SetModel: vi.fn().mockResolvedValue("Model set to openai:gpt-4"),
  SetContextLines: vi.fn().mockResolvedValue("Context set to 300 lines"),
  ForceRefresh: vi.fn().mockResolvedValue("Terminal content refreshed"),
  ExportChat: vi.fn().mockResolvedValue("/home/user/pairadmin-export.json"),
  RenameTab: vi.fn().mockResolvedValue("Tab renamed to myterm"),
}));

// Mock Wails runtime
vi.mock("../../../../wailsjs/runtime/runtime", () => ({
  EventsOn: vi.fn(() => vi.fn()),
  EventsOff: vi.fn(),
}));

// Mock useTheme
const mockSetTheme = vi.fn();
vi.mock("@/theme/theme-provider", () => ({
  useTheme: () => ({ theme: "dark", setTheme: mockSetTheme }),
}));

// Mock useLLMStream to avoid Wails dependency in hook
vi.mock("@/hooks/useLLMStream", () => ({
  useLLMStream: vi.fn(),
}));

// Helper: type and submit a command
async function sendCommand(input: HTMLElement, command: string) {
  const user = userEvent.setup();
  await user.clear(input);
  await user.type(input, command);
  await user.keyboard("{Enter}");
}

function getInput(): HTMLElement {
  return screen.getByRole("textbox");
}

function getSystemMessages(tabId = "test-tab") {
  return useChatStore.getState().messagesByTab[tabId]?.filter((m) => m.role === "system") ?? [];
}

beforeEach(() => {
  vi.clearAllMocks();
  mockSetTheme.mockClear();
  // Reset stores — initialize messagesByTab with the active tab key to avoid
  // getSnapshot returning a new [] on every selector call (Zustand infinite loop).
  useChatStore.setState({ messagesByTab: { "test-tab": [] } });
  useTerminalStore.setState({ activeTabId: "test-tab", tabs: [{ id: "test-tab", name: "tab1" }] });
});

describe("ChatPane slash command router", () => {
  it("/clear calls clearTab", async () => {
    // Seed a message first
    useChatStore.getState().addSystemMessage("test-tab", "existing message");
    expect(useChatStore.getState().messagesByTab["test-tab"]?.length).toBeGreaterThan(0);

    render(<ChatPane />);
    await sendCommand(getInput(), "/clear");

    await waitFor(() => {
      // After /clear, only the user message and cleared state remain
      const msgs = useChatStore.getState().messagesByTab["test-tab"];
      expect(msgs?.filter((m) => m.role === "system").length).toBe(0);
    });
  });

  it("/theme dark calls setTheme('dark') and adds system message", async () => {
    render(<ChatPane />);
    await sendCommand(getInput(), "/theme dark");

    await waitFor(() => {
      expect(mockSetTheme).toHaveBeenCalledWith("dark");
      const sysMessages = getSystemMessages();
      expect(sysMessages.some((m) => m.content === "Theme set to dark")).toBe(true);
    });
  });

  it("/theme light calls setTheme('light') and adds system message", async () => {
    render(<ChatPane />);
    await sendCommand(getInput(), "/theme light");

    await waitFor(() => {
      expect(mockSetTheme).toHaveBeenCalledWith("light");
      const sysMessages = getSystemMessages();
      expect(sysMessages.some((m) => m.content === "Theme set to light")).toBe(true);
    });
  });

  it("/theme invalid adds error system message", async () => {
    render(<ChatPane />);
    await sendCommand(getInput(), "/theme invalid");

    await waitFor(() => {
      expect(mockSetTheme).not.toHaveBeenCalled();
      const sysMessages = getSystemMessages();
      expect(sysMessages.some((m) => m.content.includes("Invalid theme"))).toBe(true);
    });
  });

  it("/help adds system message containing all command descriptions", async () => {
    render(<ChatPane />);
    await sendCommand(getInput(), "/help");

    await waitFor(() => {
      const sysMessages = getSystemMessages();
      expect(sysMessages.length).toBeGreaterThan(0);
      const helpMsg = sysMessages.find((m) => m.content.includes("/model"));
      expect(helpMsg).toBeDefined();
      expect(helpMsg?.content).toContain("/clear");
      expect(helpMsg?.content).toContain("/context");
      expect(helpMsg?.content).toContain("/refresh");
      expect(helpMsg?.content).toContain("/filter");
      expect(helpMsg?.content).toContain("/export");
      expect(helpMsg?.content).toContain("/rename");
      expect(helpMsg?.content).toContain("/theme");
      expect(helpMsg?.content).toContain("/help");
    });
  });

  it("/filter add test .* dispatches to LLMService.FilterCommand", async () => {
    const { FilterCommand } = await import("../../../../wailsjs/go/services/LLMService");
    render(<ChatPane />);
    await sendCommand(getInput(), "/filter add test .*");

    await waitFor(() => {
      expect(FilterCommand).toHaveBeenCalledWith("/filter add test .*");
    });
  });

  it("/model openai:gpt-4 dispatches to SettingsService.SetModel", async () => {
    const { SetModel } = await import("../../../../wailsjs/go/services/SettingsService");
    render(<ChatPane />);
    await sendCommand(getInput(), "/model openai:gpt-4");

    await waitFor(() => {
      expect(SetModel).toHaveBeenCalledWith("openai:gpt-4");
    });

    await waitFor(() => {
      const sysMessages = getSystemMessages();
      expect(sysMessages.some((m) => m.content === "Model set to openai:gpt-4")).toBe(true);
    });
  });

  it("/context 300 dispatches to SettingsService.SetContextLines(300)", async () => {
    const { SetContextLines } = await import("../../../../wailsjs/go/services/SettingsService");
    render(<ChatPane />);
    await sendCommand(getInput(), "/context 300");

    await waitFor(() => {
      expect(SetContextLines).toHaveBeenCalledWith(300);
    });

    await waitFor(() => {
      const sysMessages = getSystemMessages();
      expect(sysMessages.some((m) => m.content === "Context set to 300 lines")).toBe(true);
    });
  });

  it("/refresh dispatches to SettingsService.ForceRefresh", async () => {
    const { ForceRefresh } = await import("../../../../wailsjs/go/services/SettingsService");
    render(<ChatPane />);
    await sendCommand(getInput(), "/refresh");

    await waitFor(() => {
      expect(ForceRefresh).toHaveBeenCalled();
    });

    await waitFor(() => {
      const sysMessages = getSystemMessages();
      expect(sysMessages.some((m) => m.content === "Terminal content refreshed")).toBe(true);
    });
  });

  it("/export json dispatches to SettingsService.ExportChat with format 'json'", async () => {
    const { ExportChat } = await import("../../../../wailsjs/go/services/SettingsService");
    render(<ChatPane />);
    await sendCommand(getInput(), "/export json");

    await waitFor(() => {
      expect(ExportChat).toHaveBeenCalledWith(
        "test-tab",
        "json",
        expect.any(Array)
      );
    });
  });

  it("/rename myterm dispatches to SettingsService.RenameTab with label 'myterm'", async () => {
    const { RenameTab } = await import("../../../../wailsjs/go/services/SettingsService");
    render(<ChatPane />);
    await sendCommand(getInput(), "/rename myterm");

    await waitFor(() => {
      expect(RenameTab).toHaveBeenCalledWith("test-tab", "myterm");
    });

    await waitFor(() => {
      const sysMessages = getSystemMessages();
      expect(sysMessages.some((m) => m.content === "Tab renamed to myterm")).toBe(true);
    });
  });

  it("/unknown command falls through to normal LLM send", async () => {
    const { SendMessage } = await import("../../../../wailsjs/go/services/LLMService");
    render(<ChatPane />);
    await sendCommand(getInput(), "/unknown");

    await waitFor(() => {
      expect(SendMessage).toHaveBeenCalled();
    });
  });

  it("plain text (no slash) goes to LLM as normal", async () => {
    const { SendMessage } = await import("../../../../wailsjs/go/services/LLMService");
    render(<ChatPane />);
    await sendCommand(getInput(), "hello world");

    await waitFor(() => {
      expect(SendMessage).toHaveBeenCalled();
    });
  });
});
