import type { Metadata } from "next";
import { RootProvider } from "fumadocs-ui/provider/next";
import "./globals.css";

export const metadata: Metadata = {
  title: "CDC Lite",
  description: "Self-hosted PostgreSQL change data capture with webhook delivery.",
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return <html lang="en" suppressHydrationWarning><body className="min-h-screen"><RootProvider>{children}</RootProvider></body></html>;
}
