import Link from "next/link";
import { Activity, Radio, Settings } from "lucide-react";

const links = [
  { href: "/", label: "Overview", icon: Activity },
  { href: "/events", label: "Live Events", icon: Radio },
  { href: "/webhook", label: "Webhook Settings", icon: Settings }
];

export function Nav() {
  return <aside className="border-b border-zinc-800 bg-zinc-950 px-5 py-4 md:min-h-screen md:w-60 md:border-b-0 md:border-r">
    <div className="mb-6 text-lg font-bold tracking-tight">CDC <span className="text-emerald-400">Lite</span></div>
    <nav className="flex gap-2 md:flex-col">
      {links.map(({ href, label, icon: Icon }) => <Link key={href} href={href} className="flex items-center gap-2 rounded-md px-3 py-2 text-sm text-zinc-300 hover:bg-zinc-900 hover:text-white"><Icon size={16} />{label}</Link>)}
    </nav>
  </aside>;
}
