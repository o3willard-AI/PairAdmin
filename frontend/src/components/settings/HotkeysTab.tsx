import { useState, useRef } from "react";

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
      <label className="text-xs text-zinc-400">{label}</label>
      <input
        ref={inputRef}
        type="text"
        value={capturing ? "Press a key combination..." : value || "Not set"}
        onFocus={handleFocus}
        onBlur={handleBlur}
        onKeyDown={handleKeyDown}
        readOnly
        className="w-full bg-zinc-800 border border-zinc-700 rounded px-3 py-1.5 text-sm text-zinc-100 focus:border-zinc-500 focus:outline-none cursor-pointer"
        placeholder="Click to capture shortcut"
      />
      <p className="text-xs text-zinc-600">Click the field and press a key combination to set.</p>
    </div>
  );
}

export function HotkeysTab() {
  const [hotkeyCopyLast, setHotkeyCopyLast] = useState("");
  const [hotkeyFocusWindow, setHotkeyFocusWindow] = useState("");
  const [saveStatus, setSaveStatus] = useState<"idle" | "saving" | "saved" | "error">("idle");

  const handleSave = async () => {
    setSaveStatus("saving");
    try {
      const { SaveSettings } = await import(
        /* @vite-ignore */ "../../../wailsjs/go/services/SettingsService"
      );
      await SaveSettings({ hotkeyCopyLast, hotkeyFocusWindow });
      setSaveStatus("saved");
      setTimeout(() => setSaveStatus("idle"), 2000);
    } catch {
      setSaveStatus("error");
      setTimeout(() => setSaveStatus("idle"), 3000);
    }
  };

  return (
    <div className="space-y-4 p-6">
      <p className="text-xs text-zinc-500">
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

      <div className="flex items-center gap-3">
        <button
          onClick={handleSave}
          disabled={saveStatus === "saving"}
          className="bg-zinc-700 hover:bg-zinc-600 text-zinc-100 text-xs px-4 py-1.5 rounded disabled:opacity-50"
        >
          {saveStatus === "saving" ? "Saving..." : saveStatus === "saved" ? "Saved!" : "Save"}
        </button>
        {saveStatus === "error" && <span className="text-xs text-red-400">Save failed</span>}
      </div>
    </div>
  );
}
