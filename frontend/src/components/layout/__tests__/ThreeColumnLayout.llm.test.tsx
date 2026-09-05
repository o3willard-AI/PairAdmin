import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, waitFor } from "@testing-library/react";
import "@testing-library/jest-dom";
import { ThreeColumnLayout } from "@/components/layout/ThreeColumnLayout";
import { useSettingsStore } from "@/stores/settingsStore";

// xterm.js uses DOM APIs not available in jsdom — mock the whole module
vi.mock("@xterm/xterm", () => {
  class Terminal {
    loadAddon = vi.fn();
    open = vi.fn();
    writeln = vi.fn();
    dispose = vi.fn();
  }
  return { Terminal };
});

vi.mock("@xterm/addon-fit", () => {
  class FitAddon {
    fit = vi.fn();
  }
  return { FitAddon };
});

vi.mock("@xterm/addon-canvas", () => {
  class CanvasAddon {}
  return { CanvasAddon };
});

vi.mock("@xterm/xterm/css/xterm.css", () => ({}));

// Path resolves from frontend/src/components/layout/__tests__/ →
// frontend/wailsjs/runtime/runtime (three ../ from src/, four from src/components/layout/__tests__/).
vi.mock("../../../../wailsjs/runtime/runtime", () => ({
  EventsOn: vi.fn(() => vi.fn()),
}));

vi.mock("../../../../wailsjs/go/services/capture/CaptureManager", () => ({
  GetAdapterStatus: vi.fn(() => Promise.resolve([])),
}));

const getSettings = vi.fn();
const testConnection = vi.fn();
vi.mock("../../../../wailsjs/go/services/SettingsService", () => ({
  GetSettings: (...args: unknown[]) => getSettings(...args),
  GetAPIKeyStatus: vi.fn(() => Promise.resolve("")),
  SaveSettings: vi.fn(() => Promise.resolve(undefined)),
  SaveAPIKey: vi.fn(() => Promise.resolve(undefined)),
  TestConnection: (...args: unknown[]) => testConnection(...args),
  SetModel: vi.fn(() => Promise.resolve("")),
}));

// Mock useTheme for AppearanceTab rendered inside SettingsDialog
vi.mock("@/theme/theme-provider", () => ({
  useTheme: () => ({ theme: "dark", setTheme: vi.fn() }),
}));

beforeEach(() => {
  class ResizeObserverMock {
    observe = vi.fn();
    disconnect = vi.fn();
    unobserve = vi.fn();
  }
  global.ResizeObserver = ResizeObserverMock as unknown as typeof ResizeObserver;
  localStorage.clear();
  getSettings.mockReset().mockResolvedValue({});
  testConnection.mockReset().mockResolvedValue("Connected");
  useSettingsStore.setState({
    activeModel: "",
    settingsOpen: false,
    connectionStatus: "checking",
  });
});

describe("ThreeColumnLayout — disabled LLM startup probe", () => {
  it("sets connectionStatus to 'disabled' and never calls TestConnection when Provider is 'disabled'", async () => {
    getSettings.mockResolvedValue({ Provider: "disabled", Model: "" });

    render(
      <ThreeColumnLayout>
        <div>Chat</div>
      </ThreeColumnLayout>
    );

    await waitFor(() => {
      expect(useSettingsStore.getState().connectionStatus).toBe("disabled");
    });
    expect(testConnection).not.toHaveBeenCalled();

    // ...and a subsequent stream-derived status cannot flip it back (the
    // probe effect doesn't re-run, so nothing here re-writes the status)
    await new Promise((r) => setTimeout(r, 50));
    expect(useSettingsStore.getState().connectionStatus).toBe("disabled");
  });

  it("still probes connectivity for a real provider (connected path unchanged)", async () => {
    getSettings.mockResolvedValue({ Provider: "openai", Model: "gpt-4" });

    render(
      <ThreeColumnLayout>
        <div>Chat</div>
      </ThreeColumnLayout>
    );

    await waitFor(() => {
      expect(testConnection).toHaveBeenCalled();
    });
    await waitFor(() => {
      expect(useSettingsStore.getState().connectionStatus).toBe("connected");
    });
  });

  it("a stale in-flight probe cannot flip 'disabled' back to connected/disconnected", async () => {
    // Hold the probe's network call open so we can opt out mid-flight
    let resolveProbe: (v: string) => void = () => {};
    testConnection.mockImplementation(
      () =>
        new Promise<string>((res) => {
          resolveProbe = res;
        })
    );
    getSettings.mockResolvedValue({ Provider: "openai", Model: "gpt-4" });

    render(
      <ThreeColumnLayout>
        <div>Chat</div>
      </ThreeColumnLayout>
    );
    await waitFor(() => {
      expect(testConnection).toHaveBeenCalled();
    });

    // User saves "Disable Pair LLM" while the startup probe is still awaiting
    useSettingsStore.setState({ connectionStatus: "disabled" });

    // The probe finally comes back "Connected" — it must be discarded
    resolveProbe("Connected");
    await new Promise((r) => setTimeout(r, 20));

    expect(useSettingsStore.getState().connectionStatus).toBe("disabled");
  });
});
