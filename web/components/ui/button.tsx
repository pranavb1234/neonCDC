import type { ButtonHTMLAttributes } from "react";
import { cn } from "@/lib/utils";

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & { variant?: "primary" | "secondary" };

export function Button({ className, variant = "primary", ...props }: ButtonProps) {
  const color = variant === "secondary" ? "bg-zinc-800 text-zinc-100 hover:bg-zinc-700" : "bg-emerald-400 text-zinc-950 hover:bg-emerald-300";
  return <button className={cn("inline-flex items-center justify-center gap-2 rounded-md px-3 py-2 text-sm font-semibold transition disabled:cursor-not-allowed disabled:opacity-50", color, className)} {...props} />;
}
