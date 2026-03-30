import { Dialog } from "@base-ui/react/dialog";
import { Tabs } from "@base-ui/react/tabs";
import { LLMConfigTab } from "./LLMConfigTab";
import { PromptsTab } from "./PromptsTab";
import { TerminalsTab } from "./TerminalsTab";
import { HotkeysTab } from "./HotkeysTab";
import { AppearanceTab } from "./AppearanceTab";

const tabClass =
  "px-3 py-2 text-xs text-zinc-400 data-[selected]:text-zinc-100 data-[selected]:border-b-2 data-[selected]:border-zinc-400 cursor-pointer hover:text-zinc-300 transition-colors";

export interface SettingsDialogProps {
  open: boolean;
  onClose: () => void;
}

export function SettingsDialog({ open, onClose }: SettingsDialogProps) {
  return (
    <Dialog.Root
      open={open}
      onOpenChange={(o) => {
        if (!o) onClose();
      }}
    >
      <Dialog.Portal>
        <Dialog.Backdrop className="fixed inset-0 z-40 bg-black/60" />
        <Dialog.Popup className="fixed left-1/2 top-1/2 z-50 w-[640px] max-h-[80vh] -translate-x-1/2 -translate-y-1/2 rounded-lg bg-zinc-900 border border-zinc-700 shadow-xl flex flex-col overflow-hidden">
          <Dialog.Title className="px-6 py-4 text-sm font-semibold text-zinc-100 border-b border-zinc-800">
            Settings
          </Dialog.Title>
          <Tabs.Root defaultValue="llm-config" className="flex flex-col flex-1 overflow-hidden">
            <Tabs.List className="flex border-b border-zinc-800 px-4 flex-none">
              <Tabs.Tab value="llm-config" className={tabClass}>
                LLM Config
              </Tabs.Tab>
              <Tabs.Tab value="prompts" className={tabClass}>
                Prompts
              </Tabs.Tab>
              <Tabs.Tab value="terminals" className={tabClass}>
                Terminals
              </Tabs.Tab>
              <Tabs.Tab value="hotkeys" className={tabClass}>
                Hotkeys
              </Tabs.Tab>
              <Tabs.Tab value="appearance" className={tabClass}>
                Appearance
              </Tabs.Tab>
            </Tabs.List>
            <div className="flex-1 overflow-y-auto">
              <Tabs.Panel value="llm-config">
                <LLMConfigTab onClose={onClose} />
              </Tabs.Panel>
              <Tabs.Panel value="prompts">
                <PromptsTab />
              </Tabs.Panel>
              <Tabs.Panel value="terminals">
                <TerminalsTab />
              </Tabs.Panel>
              <Tabs.Panel value="hotkeys">
                <HotkeysTab />
              </Tabs.Panel>
              <Tabs.Panel value="appearance">
                <AppearanceTab />
              </Tabs.Panel>
            </div>
          </Tabs.Root>
        </Dialog.Popup>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
