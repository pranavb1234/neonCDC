import Link from "next/link";

export function SiteHeader() {
  return <header className="border-b border-fd-border bg-fd-background/90"><nav className="mx-auto flex max-w-6xl items-center justify-between px-6 py-4"><Link className="font-semibold" href="/">CDC <span className="text-emerald-500">Lite</span></Link><div className="flex gap-5 text-sm text-fd-muted-foreground"><Link href="/docs">Documentation</Link><a href="https://github.com/pranavb1234/neonCDC">GitHub</a></div></nav></header>;
}
