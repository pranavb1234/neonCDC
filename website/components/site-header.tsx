import Link from "next/link";
import { Terminal, BookOpen, GitBranch } from "lucide-react";

export function SiteHeader() {
  return (
    <header className="sticky top-0 z-50 border-b border-emerald-950/50 bg-[#040806]/80 backdrop-blur-md">
      <nav className="mx-auto flex max-w-7xl items-center justify-between px-6 py-4">
        <Link href="/" className="group flex items-center gap-2.5">
          <div className="relative flex h-8 w-8 items-center justify-center rounded-lg border border-emerald-500/30 bg-emerald-950/40 text-emerald-400 shadow-[0_0_15px_rgba(16,185,129,0.2)] transition-colors group-hover:border-emerald-400">
            <Terminal className="h-4 w-4" />
            <span className="absolute -top-1 -right-1 flex h-2.5 w-2.5">
              <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-emerald-400 opacity-75"></span>
              <span className="relative inline-flex h-2.5 w-2.5 rounded-full bg-emerald-500"></span>
            </span>
          </div>
          <div className="flex items-center gap-1.5">
            <span className="text-base font-semibold tracking-tight text-white">Wal</span>
            <span className="font-pixel text-xs tracking-wider text-emerald-400 uppercase">flow</span>
          </div>
        </Link>

        <div className="flex items-center gap-6 text-sm">
          <Link
            href="/docs"
            className="flex items-center gap-1.5 text-zinc-400 transition-colors hover:text-emerald-400"
          >
            <BookOpen className="h-4 w-4" />
            <span>Docs</span>
          </Link>
          <a
            href="https://github.com/pranavb1234/walflow"
            target="_blank"
            rel="noreferrer"
            className="flex items-center gap-1.5 text-zinc-400 transition-colors hover:text-emerald-400"
          >
            <svg className="h-4 w-4 fill-current" viewBox="0 0 24 24">
              <path fillRule="evenodd" clipRule="evenodd" d="M12 2C6.477 2 2 6.484 2 12.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.53 1.032 1.53 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0112 6.844c.85.004 1.705.115 2.504.337 1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.202 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.943.359.309.678.92.678 1.855 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0022 12.017C22 6.484 17.522 2 12 2z" />
            </svg>
            <span>GitHub</span>
          </a>
          <Link
            href="/docs"
            className="hidden sm:inline-flex items-center rounded-lg border border-emerald-500/30 bg-emerald-950/30 px-3.5 py-1.5 text-xs font-medium text-emerald-300 transition-colors hover:border-emerald-400/60 hover:bg-emerald-900/40"
          >
            Get Started →
          </Link>
        </div>
      </nav>
    </header>
  );
}
