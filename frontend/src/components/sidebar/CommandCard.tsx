import { Copy, RotateCw } from "lucide-react";
import type { Command } from "@/stores/commandStore";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";

interface CommandCardProps {
  command: Command;
  onCopy: (text: string) => void;
  onExecute: (text: string) => void;
}

export function CommandCard({ command, onCopy, onExecute }: CommandCardProps) {
  return (
    <TooltipProvider>
      <Tooltip>
        {/* Renders as a plain div, not a button — this is the card itself.
            It intentionally has no click action: clicking it will be used
            for drag-and-drop reordering, not for copy/execute. Only the two
            icon buttons below trigger terminal actions. */}
        <TooltipTrigger
          render={<div />}
          data-testid="command-card"
          className="group w-full text-left px-3 py-2 text-xs font-mono bg-zinc-900 hover:bg-zinc-800 rounded border border-zinc-800 hover:border-zinc-700 transition-colors flex items-center gap-1"
        >
          <span className="truncate flex-1">{command.command}</span>
          <button
            type="button"
            onClick={(e) => {
              e.stopPropagation();
              onCopy(command.command);
            }}
            aria-label="Copy to Terminal"
            className="flex-none opacity-0 group-hover:opacity-100 transition-opacity text-zinc-400 hover:text-zinc-200"
          >
            <Copy size={12} />
          </button>
          <button
            type="button"
            onClick={(e) => {
              e.stopPropagation();
              onExecute(command.command);
            }}
            aria-label="Execute in Terminal"
            className="flex-none opacity-0 group-hover:opacity-100 transition-opacity text-zinc-400 hover:text-zinc-200"
          >
            <RotateCw size={12} />
          </button>
        </TooltipTrigger>
        <TooltipContent side="left" className="max-w-[200px]">
          <p className="text-xs text-zinc-400 mb-0.5">Generated from:</p>
          <p className="text-xs">{command.originalQuestion}</p>
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  );
}
