import { Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";

interface ClearHistoryButtonProps {
  onClick: () => void;
}

export function ClearHistoryButton({ onClick }: ClearHistoryButtonProps) {
  return (
    <Button
      variant="ghost"
      size="sm"
      onClick={onClick}
      className="w-full text-xs text-zinc-500 hover:text-zinc-300"
    >
      <Trash2 size={14} />
      Clear History
    </Button>
  );
}
