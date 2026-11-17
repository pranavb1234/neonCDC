import type { Metadata } from "next";
import "./globals.css";
import { Nav } from "@/components/nav";

export const metadata: Metadata = { title: "Walflow", description: "Local PostgreSQL CDC dashboard" };

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return <html lang="en"><body><div className="min-h-screen md:flex"><Nav /><main className="mx-auto w-full max-w-6xl p-6 md:p-10">{children}</main></div></body></html>;
}
