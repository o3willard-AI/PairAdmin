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
      className="w-full text-xs text-surface-text-muted hover:text-surface-text"
    >
      <Trash2 size={14} />
      Clear History
    </Button>
  );
}
