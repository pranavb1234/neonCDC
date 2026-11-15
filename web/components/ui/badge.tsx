import { cn } from "@/lib/utils";

export function Badge({ children, variant = "neutral" }: { children: React.ReactNode; variant?: "insert" | "update" | "delete" | "neutral" }) {
  const colors = { insert: "bg-emerald-500/15 text-emerald-300", update: "bg-amber-400/15 text-amber-200", delete: "bg-rose-500/15 text-rose-200", neutral: "bg-zinc-800 text-zinc-300" };
  return <span className={cn("rounded-full px-2 py-1 text-xs font-medium", colors[variant])}>{children}</span>;
}
