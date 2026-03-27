import { Copy } from "lucide-react";
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
}

export function CommandCard({ command, onCopy }: CommandCardProps) {
  return (
    <TooltipProvider>
      <Tooltip>
        <TooltipTrigger
          onClick={() => onCopy(command.command)}
          className="group w-full text-left px-3 py-2 text-xs font-mono bg-zinc-900 hover:bg-zinc-800 rounded border border-zinc-800 hover:border-zinc-700 transition-colors flex items-center gap-1"
        >
          <span className="truncate flex-1">{command.command}</span>
          <Copy
            size={12}
            className="flex-none opacity-0 group-hover:opacity-100 transition-opacity text-zinc-400"
          />
        </TooltipTrigger>
        <TooltipContent side="left" className="max-w-[200px]">
          <p className="text-xs text-zinc-400 mb-0.5">Generated from:</p>
          <p className="text-xs">{command.originalQuestion}</p>
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  );
}
