import { useRef } from "react";
import { useTerminalStore } from "@/stores/terminalStore";
import { useChatStore } from "@/stores/chatStore";
import { useLLMStream } from "@/hooks/useLLMStream";
import { useTheme } from "@/theme/theme-provider";
import { useSettingsStore } from "@/stores/settingsStore";
import { readTerminalLines } from "@/utils/terminalContext";
import { ChatMessageList } from "./ChatMessageList";
import { ChatInput } from "./ChatInput";

const HELP_TEXT = `/clear - Clear chat history for current tab
/model <provider:model> - Switch LLM provider and model (e.g., /model openai:gpt-4)
/context <lines> - Set terminal context window size
/refresh - Force re-capture terminal content
/filter add|list|remove - Manage sensitive data filter patterns
/export json|txt - Export current session chat history
/rename <label> - Rename current terminal tab
/theme dark|light - Switch color scheme
/help - Show this help message`;

export function ChatPane() {
  const activeTabId = useTerminalStore((state) => state.activeTabId);
  const lastSentRef = useRef<{ text: string; terminalContext: string } | null>(null);
  const { setTheme } = useTheme();

  // Subscribe to Wails streaming events for this tab
  useLLMStream(activeTabId);

  const handleSend = async (text: string) => {
    useChatStore.getState().addUserMessage(activeTabId, text);
    const trimmed = text.trim();

    // /clear — frontend only
    if (trimmed === "/clear") {
      useChatStore.getState().clearTab(activeTabId);
      return;
    }

    // /theme — frontend only per D-07
    if (trimmed.startsWith("/theme ")) {
      const t = trimmed.slice(7).trim();
      if (t === "dark" || t === "light") {
        setTheme(t);
        useChatStore.getState().addSystemMessage(activeTabId, `Theme set to ${t}`);
      } else {
        useChatStore.getState().addSystemMessage(activeTabId, `Invalid theme: "${t}". Use: /theme dark|light`);
      }
      return;
    }

    // /help — frontend only per D-07
    if (trimmed === "/help") {
      useChatStore.getState().addSystemMessage(activeTabId, HELP_TEXT);
      return;
    }

    // /filter — Go backend call (LLMService.FilterCommand)
    if (trimmed.startsWith("/filter")) {
      import(/* @vite-ignore */ "../../../wailsjs/go/services/LLMService")
        .then(({ FilterCommand }) => FilterCommand(trimmed))
        .then((response: string) => {
          useChatStore.getState().addSystemMessage(activeTabId, response);
        })
        .catch((err: Error) => {
          useChatStore.getState().addSystemMessage(
            activeTabId,
            `Filter error: ${err?.message ?? "Unknown error"}`
          );
        });
      return;
    }

    // /model — Go backend call per D-07
    if (trimmed.startsWith("/model ")) {
      const arg = trimmed.slice(7).trim();
      import(/* @vite-ignore */ "../../../wailsjs/go/services/SettingsService")
        .then(({ SetModel }) => SetModel(arg))
        .then((response: string) => {
          useChatStore.getState().addSystemMessage(activeTabId, response);
          useSettingsStore.getState().setActiveModel(arg);
        })
        .catch((err: Error) => {
          useChatStore.getState().addSystemMessage(activeTabId, `Error: ${err?.message ?? "Unknown error"}`);
        });
      return;
    }

    // /context — Go backend call per D-07
    if (trimmed.startsWith("/context ")) {
      const lines = parseInt(trimmed.slice(9).trim(), 10);
      if (isNaN(lines)) {
        useChatStore.getState().addSystemMessage(activeTabId, "Usage: /context <number>");
        return;
      }
      import(/* @vite-ignore */ "../../../wailsjs/go/services/SettingsService")
        .then(({ SetContextLines }) => SetContextLines(lines))
        .then((response: string) => {
          useChatStore.getState().addSystemMessage(activeTabId, response);
        })
        .catch((err: Error) => {
          useChatStore.getState().addSystemMessage(activeTabId, `Error: ${err?.message ?? "Unknown error"}`);
        });
      return;
    }

    // /refresh — Go backend call per D-07
    if (trimmed === "/refresh") {
      import(/* @vite-ignore */ "../../../wailsjs/go/services/SettingsService")
        .then(({ ForceRefresh }) => ForceRefresh())
        .then((response: string) => {
          useChatStore.getState().addSystemMessage(activeTabId, response);
        })
        .catch((err: Error) => {
          useChatStore.getState().addSystemMessage(activeTabId, `Error: ${err?.message ?? "Unknown error"}`);
        });
      return;
    }

    // /export — Go backend call per D-07
    if (trimmed.startsWith("/export")) {
      const format = trimmed.slice(7).trim() || "json";
      if (format !== "json" && format !== "txt") {
        useChatStore.getState().addSystemMessage(activeTabId, "Usage: /export json|txt");
        return;
      }
      const messages = useChatStore.getState().messagesByTab[activeTabId] || [];
      const exportMsgs = messages.map((m) => ({ role: m.role, content: m.content }));
      import(/* @vite-ignore */ "../../../wailsjs/go/services/SettingsService")
        .then(({ ExportChat }) => ExportChat(activeTabId, format, exportMsgs))
        .then((response: string) => {
          useChatStore.getState().addSystemMessage(activeTabId, `Exported to: ${response}`);
        })
        .catch((err: Error) => {
          useChatStore.getState().addSystemMessage(activeTabId, `Export error: ${err?.message ?? "Unknown error"}`);
        });
      return;
    }

    // /rename — Go backend call per D-07
    if (trimmed.startsWith("/rename ")) {
      const label = trimmed.slice(8).trim();
      if (!label) {
        useChatStore.getState().addSystemMessage(activeTabId, "Usage: /rename <label>");
        return;
      }
      import(/* @vite-ignore */ "../../../wailsjs/go/services/SettingsService")
        .then(({ RenameTab }) => RenameTab(activeTabId, label))
        .then((response: string) => {
          useChatStore.getState().addSystemMessage(activeTabId, response);
        })
        .catch((err: Error) => {
          useChatStore.getState().addSystemMessage(activeTabId, `Error: ${err?.message ?? "Unknown error"}`);
        });
      return;
    }

    // Not a command — send to LLM (existing behavior)
    const term = useTerminalStore.getState().getTermRef(activeTabId);
    const terminalContext = readTerminalLines(term, 200);
    lastSentRef.current = { text, terminalContext };

    import(/* @vite-ignore */ "../../../wailsjs/go/services/LLMService").then(({ SendMessage }) => {
      SendMessage(activeTabId, text, terminalContext).catch((err: Error) => {
        useChatStore.getState().setStreamError(activeTabId, null, err?.message ?? "Failed to send message");
      });
    }).catch(() => {
      useChatStore.getState().addAssistantMessage(activeTabId, "[Dev mode: Wails runtime unavailable]");
    });
  };

  const handleRetry = () => {
    if (lastSentRef.current) {
      handleSend(lastSentRef.current.text);
    }
  };

  return (
    <div className="flex flex-1 flex-col overflow-hidden">
      <ChatMessageList onRetry={handleRetry} />
      <ChatInput onSend={handleSend} />
    </div>
  );
}
