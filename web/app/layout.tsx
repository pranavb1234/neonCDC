import type { Metadata } from "next";
import { RootProvider } from "fumadocs-ui/provider/next";
import "./globals.css";
import { AppShell } from "@/components/app-shell";

export const metadata: Metadata = { title: "CDC Lite", description: "Local PostgreSQL CDC dashboard" };

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return <html lang="en" suppressHydrationWarning><body><RootProvider><AppShell>{children}</AppShell></RootProvider></body></html>;
}
