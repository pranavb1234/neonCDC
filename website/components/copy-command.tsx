"use client";

import { useState } from "react";
import { Check, Copy } from "lucide-react";

export function CopyCommand({ command }: { command: string }) {
  const [copied, setCopied] = useState(false);

  const handleCopy = () => {
    navigator.clipboard.writeText(command);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="group relative flex items-center gap-3 rounded-xl border border-emerald-900/60 bg-[#020503] px-5 py-3.5 font-mono-code text-sm text-emerald-400/90 shadow-lg">
      <span className="text-emerald-500 select-none">$</span>
      <span className="flex-1 truncate text-zinc-300">{command}</span>
      <button
        onClick={handleCopy}
        className="shrink-0 rounded-md p-1.5 text-zinc-500 hover:text-emerald-400 hover:bg-emerald-950/40 transition-colors"
        aria-label="Copy command"
      >
        {copied ? (
          <Check className="h-4 w-4 text-emerald-400" />
        ) : (
          <Copy className="h-4 w-4" />
        )}
      </button>
    </div>
  );
}
