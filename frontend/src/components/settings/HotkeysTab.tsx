import { useEffect, useState, useRef } from "react";
import { mergeAndSaveSettings } from "@/utils/settingsSync";
import { DEFAULT_ADD_CLIPBOARD_COMMAND_HOTKEY } from "@/hooks/useAddClipboardCommandHotkey";
import { DEFAULT_NEW_TERMINAL_HOTKEY } from "@/hooks/useNewTerminalHotkey";
import { DEFAULT_ADD_COMMAND_HOTKEY } from "@/hooks/useAddCommandHotkey";
import { DEFAULT_QUICK_SELECT_CHORD } from "@/hooks/useQuickSelect";

function buildKeyCombo(event: KeyboardEvent): string {
  const parts: string[] = [];
  if (event.ctrlKey) parts.push("Ctrl");
  if (event.shiftKey) parts.push("Shift");
  if (event.altKey) parts.push("Alt");
  if (event.metaKey) parts.push("Meta");
  // Exclude modifier keys themselves as the primary key
  const key = event.key;
  if (!["Control", "Shift", "Alt", "Meta"].includes(key)) {
    parts.push(key.length === 1 ? key.toUpperCase() : key);
  }
  return parts.join("+");
}

interface HotkeyInputProps {
  label: string;
  value: string;
  onChange: (combo: string) => void;
}

function HotkeyInput({ label, value, onChange }: HotkeyInputProps) {
  const [capturing, setCapturing] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);

  const handleFocus = () => {
    setCapturing(true);
  };

  const handleBlur = () => {
    setCapturing(false);
  };

  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    e.preventDefault();
    const combo = buildKeyCombo(e.nativeEvent);
    if (combo) {
      onChange(combo);
      inputRef.current?.blur();
    }
  };

  return (
    <div className="space-y-1">
      <label className="text-xs text-surface-text-muted">{label}</label>
      <input
        ref={inputRef}
        type="text"
        value={capturing ? "Press a key combination..." : value || "Not set"}
        onFocus={handleFocus}
        onBlur={handleBlur}
        onKeyDown={handleKeyDown}
        readOnly
        className="w-full bg-surface-2 border border-surface-border-strong rounded px-3 py-1.5 text-sm text-surface-text focus:border-surface-text-muted focus:outline-none cursor-pointer"
        placeholder="Click to capture shortcut"
      />
      <p className="text-xs text-surface-text-muted">Click the field and press a key combination to set.</p>
    </div>
  );
}

export function HotkeysTab() {
  const [hotkeyCopyLast, setHotkeyCopyLast] = useState("");
  const [hotkeyFocusWindow, setHotkeyFocusWindow] = useState("");
  const [hotkeyAddClipboardCommand, setHotkeyAddClipboardCommand] = useState(
    DEFAULT_ADD_CLIPBOARD_COMMAND_HOTKEY
  );
  const [hotkeyNewTerminal, setHotkeyNewTerminal] = useState(DEFAULT_NEW_TERMINAL_HOTKEY);
  const [hotkeyAddCommand, setHotkeyAddCommand] = useState(DEFAULT_ADD_COMMAND_HOTKEY);
  const [hotkeyQuickSelect, setHotkeyQuickSelect] = useState(DEFAULT_QUICK_SELECT_CHORD);
  const [saveStatus, setSaveStatus] = useState<"idle" | "saving" | "saved" | "error">("idle");

  useEffect(() => {
    import(/* @vite-ignore */ "../../../wailsjs/go/services/SettingsService")
      .then(({ GetSettings }) => GetSettings())
      .then((cfg) => {
        if (cfg.HotkeyCopyLast) setHotkeyCopyLast(cfg.HotkeyCopyLast);
        if (cfg.HotkeyFocusWindow) setHotkeyFocusWindow(cfg.HotkeyFocusWindow);
        if (cfg.HotkeyAddClipboardCommand) setHotkeyAddClipboardCommand(cfg.HotkeyAddClipboardCommand);
        if (cfg.HotkeyNewTerminal) setHotkeyNewTerminal(cfg.HotkeyNewTerminal);
        if (cfg.HotkeyAddCommand) setHotkeyAddCommand(cfg.HotkeyAddCommand);
        if (cfg.HotkeyQuickSelect) setHotkeyQuickSelect(cfg.HotkeyQuickSelect);
      })
      .catch(() => {});
  }, []);

  const handleSave = async () => {
    setSaveStatus("saving");
    try {
      await mergeAndSaveSettings({
        HotkeyCopyLast: hotkeyCopyLast,
        HotkeyFocusWindow: hotkeyFocusWindow,
        HotkeyAddClipboardCommand: hotkeyAddClipboardCommand,
        HotkeyNewTerminal: hotkeyNewTerminal,
        HotkeyAddCommand: hotkeyAddCommand,
        HotkeyQuickSelect: hotkeyQuickSelect,
      });
      setSaveStatus("saved");
      setTimeout(() => setSaveStatus("idle"), 2000);
    } catch {
      setSaveStatus("error");
      setTimeout(() => setSaveStatus("idle"), 3000);
    }
  };

  return (
    <div className="space-y-4 p-6">
      <p className="text-xs text-surface-text-muted">
        In-app keyboard shortcuts (window must be focused). Global hotkeys are not supported.
      </p>

      <HotkeyInput
        label="Copy Last Command"
        value={hotkeyCopyLast}
        onChange={setHotkeyCopyLast}
      />

      <HotkeyInput
        label="Focus PairAdmin Window"
        value={hotkeyFocusWindow}
        onChange={setHotkeyFocusWindow}
      />

      <div className="space-y-1">
        <HotkeyInput
          label="New Terminal"
          value={hotkeyNewTerminal}
          onChange={setHotkeyNewTerminal}
        />
        <p className="text-xs text-surface-text-muted">
          Opens the "+ New" terminal dialog without needing to click it.
        </p>
      </div>

      <div className="space-y-1">
        <HotkeyInput
          label="Add Clipboard as Command"
          value={hotkeyAddClipboardCommand}
          onChange={setHotkeyAddClipboardCommand}
        />
        <p className="text-xs text-surface-text-muted">
          Grabs whatever is currently on the clipboard and adds it as a new command in the
          sidebar — copy a command in the terminal, then press this to save it.
        </p>
      </div>

      <div className="space-y-1">
        <HotkeyInput
          label="Add Command"
          value={hotkeyAddCommand}
          onChange={setHotkeyAddCommand}
        />
        <p className="text-xs text-surface-text-muted">
          Opens the "Add Command" dialog in the sidebar without needing to click the button.
        </p>
      </div>

      <div className="space-y-1">
        <HotkeyInput
          label="Quick Select"
          value={hotkeyQuickSelect}
          onChange={setHotkeyQuickSelect}
        />
        <p className="text-xs text-surface-text-muted">
          Hold this chord, then press F1–F12
        </p>
      </div>

      <div className="flex items-center gap-3">
        <button
          onClick={handleSave}
          disabled={saveStatus === "saving"}
          className="bg-surface-3 hover:bg-surface-3/80 text-surface-text text-xs px-4 py-1.5 rounded disabled:opacity-50"
        >
          {saveStatus === "saving" ? "Saving..." : saveStatus === "saved" ? "Saved!" : "Save"}
        </button>
        {saveStatus === "error" && <span className="text-xs text-red-400">Save failed</span>}
      </div>
    </div>
  );
}
