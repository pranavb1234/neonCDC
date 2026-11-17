import type { Metadata } from "next";
import { RootProvider } from "fumadocs-ui/provider/next";
import "./globals.css";

export const metadata: Metadata = {
  title: "Walflow — Every CHANGE has a destination",
  description: "Capture every change in your PostgreSQL database and stream it wherever it needs to go. Self-hosted, reliable, and completely under your control.",
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en" suppressHydrationWarning>
      <head>
        <link rel="preconnect" href="https://fonts.googleapis.com" />
        <link rel="preconnect" href="https://fonts.gstatic.com" crossOrigin="anonymous" />
      </head>
      <body className="min-h-screen bg-[#040806] text-zinc-100 antialiased">
        <RootProvider theme={{ defaultTheme: "dark", forcedTheme: "dark", enableSystem: false }}>{children}</RootProvider>
      </body>
    </html>
  );
}
