import { Settings } from "lucide-react";

export function StatusBar() {
  return (
    <div className="h-7 flex-none flex items-center px-3 text-xs text-zinc-500 bg-zinc-900 border-t border-zinc-800 gap-4">
      {/* Left: model indicator */}
      <div className="flex items-center gap-1.5">
        <span className="inline-block w-1.5 h-1.5 rounded-full bg-zinc-600" />
        <span>No model</span>
      </div>

      {/* Center: connection status */}
      <div className="flex-1 text-center">
        <span>Disconnected</span>
      </div>

      {/* Right: token meter */}
      <div className="flex items-center gap-3">
        <span>0 / 0 tokens</span>
        <button className="hover:text-zinc-300 transition-colors" disabled>
          <Settings size={14} />
        </button>
      </div>
    </div>
  );
}
