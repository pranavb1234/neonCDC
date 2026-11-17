"use client";

import { usePathname } from "next/navigation";
import { Nav } from "@/components/nav";

export function AppShell({ children }: { children: React.ReactNode }) {
  const isDocs = usePathname().startsWith("/docs");
  if (isDocs) return <>{children}</>;
  return <div className="min-h-screen md:flex"><Nav /><main className="mx-auto w-full max-w-6xl p-6 md:p-10">{children}</main></div>;
}
