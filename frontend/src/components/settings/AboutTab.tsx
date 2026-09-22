import { useEffect, useState } from "react";

export function AboutTab() {
  const [version, setVersion] = useState<string | null>(null);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    import(/* @vite-ignore */ "../../../wailsjs/go/services/SettingsService")
      .then(({ GetVersion }) => GetVersion())
      .then((v) => {
        if (v && v.trim() !== "") setVersion(v);
        else setFailed(true);
      })
      .catch(() => setFailed(true));
  }, []);

  return (
    <div className="space-y-4 p-6">
      <div className="space-y-2">
        <label className="text-xs text-surface-text-muted">Application</label>
        <p className="text-sm text-surface-text">PairAdmin</p>
      </div>
      <div className="space-y-2">
        <label className="text-xs text-surface-text-muted">Version</label>
        <p className="text-sm text-surface-text">
          {failed
            ? "version unavailable"
            : version === null
              ? "\u2026"
              : version}
        </p>
      </div>
    </div>
  );
}