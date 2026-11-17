import Link from "next/link";
import { SiteHeader } from "@/components/site-header";
import { HeroCircuit } from "@/components/hero-circuit";
import { ArrowRight } from "lucide-react";

export default function LandingPage() {
  return (
    <div className="relative min-h-screen bg-[#040806] text-zinc-100 selection:bg-emerald-500/30 selection:text-emerald-200">
      <SiteHeader />

      {/* Hero Section */}
      <section className="relative overflow-hidden pt-16 pb-24 md:pt-32 md:pb-36 tech-grid-pattern">
        {/* Background circuit graphics */}
        <HeroCircuit />

        <div className="relative z-10 mx-auto max-w-7xl px-6">
          {/* Main Headline */}
          <h1 className="text-4xl sm:text-6xl md:text-7xl lg:text-[5.25rem] font-bold tracking-tight text-white leading-[1.12] max-w-4xl">
            Every{" "}
            <span className="font-pixel text-emerald-400 tracking-wide uppercase select-none inline-block">
              CHANGE
            </span>
            <br />
            <span className="text-zinc-200">has a destination.</span>
          </h1>

          {/* Subtitle */}
          <p className="mt-7 max-w-2xl text-base sm:text-lg md:text-xl text-zinc-300 font-normal leading-relaxed">
            Capture every change in your PostgreSQL database and stream it wherever it needs to go. Self-hosted, reliable, and completely under your control.
          </p>

          {/* Call to Action */}
          <div className="mt-9 flex items-center">
            <Link
              href="/docs"
              className="glow-pill-green inline-flex items-center gap-2.5 rounded-xl px-8 py-3.5 text-sm sm:text-base font-semibold tracking-wide"
            >
              <span>Get started</span>
              <ArrowRight className="h-4 w-4" />
            </Link>
          </div>
        </div>
      </section>

      {/* Quickstart Terminal Section */}
      <section className="relative mx-auto max-w-7xl px-6 py-16 md:py-24 border-t border-emerald-950/40">
        <div className="rounded-2xl border border-emerald-950/90 bg-[#050b07] p-8 md:p-12 relative overflow-hidden">
          <div className="absolute top-0 right-0 w-96 h-96 bg-emerald-500/10 blur-[100px] rounded-full pointer-events-none" />
          
          <div className="relative z-10 flex flex-col lg:flex-row lg:items-center lg:justify-between gap-8">
            <div className="max-w-xl">
              <span className="font-pixel text-xs text-emerald-400 uppercase tracking-widest">[ DEPLOY_NOW ]</span>
              <h2 className="mt-2 text-2xl sm:text-3xl font-bold tracking-tight text-white">
                Spin up in less than 60 seconds
              </h2>
              <p className="mt-3 text-sm text-zinc-400 leading-relaxed">
                Run Walflow locally with Docker Compose or embed it in your Kubernetes cluster. Connect any existing PostgreSQL 12+ database with a single publication.
              </p>
              <div className="mt-6 flex flex-wrap gap-4">
                <Link
                  href="/docs"
                  className="rounded-lg bg-emerald-500 px-5 py-2.5 text-xs font-semibold text-black hover:bg-emerald-400 transition-colors"
                >
                  Read Installation Guide
                </Link>
                <a
                  href="https://github.com/pranavb1234/walflow"
                  target="_blank"
                  rel="noreferrer"
                  className="rounded-lg border border-emerald-500/30 px-5 py-2.5 text-xs font-medium text-emerald-300 hover:bg-emerald-950/40 transition-colors"
                >
                  GitHub Repository
                </a>
              </div>
            </div>

            {/* Code Terminal Box */}
            <div className="w-full lg:max-w-md rounded-xl border border-emerald-900/60 bg-[#020503] p-4 font-mono-code text-xs text-emerald-400/90 shadow-2xl">
              <div className="flex items-center gap-2 border-b border-zinc-800/80 pb-3 mb-3 text-zinc-500 text-[11px]">
                <div className="flex gap-1.5">
                  <div className="h-2.5 w-2.5 rounded-full bg-red-500/40" />
                  <div className="h-2.5 w-2.5 rounded-full bg-yellow-500/40" />
                  <div className="h-2.5 w-2.5 rounded-full bg-green-500/40" />
                </div>
                <span className="ml-2 text-zinc-400">bash — terminal</span>
              </div>
              <div className="space-y-1.5 text-zinc-300">
                <p className="text-zinc-500"># Clone and start with Docker</p>
                <p><span className="text-emerald-500">$</span> git clone https://github.com/pranavb1234/walflow.git</p>
                <p><span className="text-emerald-500">$</span> cd walflow</p>
                <p><span className="text-emerald-500">$</span> docker compose up -d</p>
                <p className="pt-2 text-emerald-400 font-pixel text-[11px]">&gt; [OK] Walflow streaming from postgres://localhost:5432</p>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* Footer */}
      <footer className="border-t border-emerald-950/50 bg-[#030604] py-12">
        <div className="mx-auto flex max-w-7xl flex-col sm:flex-row items-center justify-between gap-6 px-6 text-xs text-zinc-400">
          <div className="flex items-center gap-2.5">
            <span className="font-semibold text-white">Wal<span className="font-pixel text-emerald-400">flow</span></span>
            <span>—</span>
            <span>Self-hosted PostgreSQL Change Data Capture</span>
          </div>

          <div className="flex items-center gap-6">
            <Link href="/docs" className="hover:text-emerald-300 transition-colors">
              Documentation
            </Link>
            <Link href="/docs/pipelines" className="hover:text-emerald-300 transition-colors">
              Pipelines
            </Link>
            <Link href="/docs/operations" className="hover:text-emerald-300 transition-colors">
              Operations
            </Link>
            <a
              href="https://github.com/pranavb1234/walflow"
              target="_blank"
              rel="noreferrer"
              className="hover:text-emerald-300 transition-colors"
            >
              GitHub
            </a>
          </div>
        </div>
      </footer>
    </div>
  );
}
