"use client";

import { useEffect, useState } from "react";
import { Card } from "@/components/ui/card";

type Overview = { eventCount: number; pending: number; consumers: number; webhookUrl: string; error?: string };

export function Overview() {
  const [data, setData] = useState<Overview>();
  useEffect(() => {
    const load = () => fetch("/api/overview").then((response) => response.json()).then(setData).catch(() => setData({ eventCount: 0, pending: 0, consumers: 0, webhookUrl: "Not configured", error: "Could not load overview" }));
    load(); const timer = setInterval(load, 2000); return () => clearInterval(timer);
  }, []);
  const cards = [{ label: "Events observed", value: data?.eventCount ?? "—" }, { label: "Pending deliveries", value: data?.pending ?? "—" }, { label: "Delivery consumers", value: data?.consumers ?? "—" }];
  return <><div className="mb-8"><p className="text-sm text-emerald-400">Local CDC monitor</p><h1 className="mt-1 text-3xl font-bold">Overview</h1><p className="mt-2 text-zinc-400">PostgreSQL changes buffered in Redis and delivered to your webhook.</p></div><div className="grid gap-4 md:grid-cols-3">{cards.map((card) => <Card key={card.label}><p className="text-sm text-zinc-400">{card.label}</p><p className="mt-3 text-3xl font-semibold">{card.value}</p></Card>)}</div><Card className="mt-4"><p className="text-sm text-zinc-400">Webhook destination</p><p className="mt-2 break-all font-mono text-sm">{data?.webhookUrl ?? "Loading…"}</p><p className="mt-3 text-sm text-zinc-500">{data?.error ?? (data?.pending ? "Events are waiting for a successful webhook response." : "Delivery status is healthy.")}</p></Card></>;
}
