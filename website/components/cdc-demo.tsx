"use client";

import { useState } from "react";
import { Play, Check, Copy, ArrowRight, ShieldCheck, Database, Server, Send } from "lucide-react";

interface CDCEvent {
  id: string;
  table: string;
  action: "INSERT" | "UPDATE" | "DELETE";
  lsn: string;
  timestamp: string;
  payload: Record<string, any>;
  signature: string;
  status: "delivered" | "buffering" | "acknowledged";
}

const sampleEvents: CDCEvent[] = [
  {
    id: "evt_9a4f21",
    table: "orders",
    action: "UPDATE",
    lsn: "0/16B2378",
    timestamp: "2026-11-18T00:24:12Z",
    payload: { id: 4821, customer_id: "cus_8892", total_usd: 184.5, status: "shipped" },
    signature: "sha256=9b72a4c18f3...",
    status: "acknowledged",
  },
  {
    id: "evt_3b8e72",
    table: "users",
    action: "INSERT",
    lsn: "0/16B2390",
    timestamp: "2026-11-18T00:24:14Z",
    payload: { id: 9104, email: "alex@acme.dev", plan: "pro", verified: true },
    signature: "sha256=e14a82df99c...",
    status: "delivered",
  },
  {
    id: "evt_7c1d44",
    table: "inventory",
    action: "UPDATE",
    lsn: "0/16B2414",
    timestamp: "2026-11-18T00:24:16Z",
    payload: { sku: "SKU-992", warehouse: "us-east-1", stock_count: 42 },
    signature: "sha256=22c4f97ab01...",
    status: "delivered",
  },
];

export function CDCDemo() {
  const [events, setEvents] = useState<CDCEvent[]>(sampleEvents);
  const [selectedEvent, setSelectedEvent] = useState<CDCEvent>(sampleEvents[0]);
  const [copied, setCopied] = useState(false);
  const [isSimulating, setIsSimulating] = useState(false);

  const simulateNewChange = () => {
    setIsSimulating(true);
    const actions: ("INSERT" | "UPDATE" | "DELETE")[] = ["INSERT", "UPDATE", "DELETE"];
    const tables = ["customers", "payments", "subscriptions", "audit_log"];
    const chosenAction = actions[Math.floor(Math.random() * actions.length)];
    const chosenTable = tables[Math.floor(Math.random() * tables.length)];
    const randomHex = Math.random().toString(16).substring(2, 8).toUpperCase();
    
    const newEvent: CDCEvent = {
      id: `evt_${Math.random().toString(16).substring(2, 8)}`,
      table: chosenTable,
      action: chosenAction,
      lsn: `0/16B${randomHex}`,
      timestamp: new Date().toISOString(),
      payload: {
        id: Math.floor(1000 + Math.random() * 9000),
        entity: chosenTable,
        action: chosenAction,
        modified_at: new Date().toISOString(),
      },
      signature: `sha256=${Math.random().toString(16).substring(2, 12)}...`,
      status: "delivered",
    };

    setTimeout(() => {
      setEvents((prev) => [newEvent, ...prev.slice(0, 4)]);
      setSelectedEvent(newEvent);
      setIsSimulating(false);
    }, 350);
  };

  const copyJson = () => {
    navigator.clipboard.writeText(JSON.stringify(selectedEvent, null, 2));
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="relative rounded-2xl border border-emerald-950/80 bg-[#060c09]/90 p-5 md:p-7 shadow-[0_0_50px_rgba(0,0,0,0.8)] backdrop-blur-xl">
      {/* Top subtle pipeline stream header */}
      <div className="flex flex-wrap items-center justify-between gap-4 border-b border-emerald-950/60 pb-5">
        <div className="flex items-center gap-3">
          <div className="flex h-3 w-3 items-center justify-center">
            <span className="relative flex h-2 w-2">
              <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-emerald-400 opacity-75"></span>
              <span className="relative inline-flex h-2 w-2 rounded-full bg-emerald-500"></span>
            </span>
          </div>
          <div>
            <h3 className="text-sm font-semibold tracking-wide text-zinc-100 flex items-center gap-2">
              <span>Live Replication Stream</span>
              <span className="font-pixel text-[10px] text-emerald-400 border border-emerald-500/30 px-1.5 py-0.5 rounded bg-emerald-950/40">
                pgoutput
              </span>
            </h3>
            <p className="text-xs text-zinc-400">Capturing WAL records & delivering signed webhooks</p>
          </div>
        </div>

        <button
          onClick={simulateNewChange}
          disabled={isSimulating}
          className="flex items-center gap-2 rounded-lg border border-emerald-500/40 bg-emerald-950/50 px-3.5 py-1.5 text-xs font-medium text-emerald-300 transition-all hover:bg-emerald-900/60 hover:border-emerald-400 active:scale-95 disabled:opacity-50"
        >
          <Play className={`h-3.5 w-3.5 ${isSimulating ? "animate-spin text-emerald-400" : "fill-emerald-400"}`} />
          <span>{isSimulating ? "Streaming..." : "Simulate DB Write"}</span>
        </button>
      </div>

      {/* 3-step visual trace */}
      <div className="my-5 grid grid-cols-1 md:grid-cols-3 gap-3">
        <div className="flex items-center gap-3 rounded-lg border border-emerald-900/30 bg-[#040806]/80 p-3">
          <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded bg-emerald-950/60 text-emerald-400 border border-emerald-800/40">
            <Database className="h-4 w-4" />
          </div>
          <div className="overflow-hidden">
            <p className="text-[10px] font-mono-code text-zinc-400 uppercase tracking-wider">1. PostgreSQL WAL</p>
            <p className="truncate text-xs font-medium text-zinc-200">Replication slot active</p>
          </div>
        </div>

        <div className="flex items-center gap-3 rounded-lg border border-emerald-900/30 bg-[#040806]/80 p-3">
          <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded bg-emerald-950/60 text-emerald-400 border border-emerald-800/40">
            <Server className="h-4 w-4" />
          </div>
          <div className="overflow-hidden">
            <p className="text-[10px] font-mono-code text-zinc-400 uppercase tracking-wider">2. Redis Stream</p>
            <p className="truncate text-xs font-medium text-zinc-200">LSN acknowledged buffer</p>
          </div>
        </div>

        <div className="flex items-center gap-3 rounded-lg border border-emerald-900/30 bg-[#040806]/80 p-3">
          <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded bg-emerald-950/60 text-emerald-400 border border-emerald-800/40">
            <Send className="h-4 w-4" />
          </div>
          <div className="overflow-hidden">
            <p className="text-[10px] font-mono-code text-zinc-400 uppercase tracking-wider">3. Signed Webhook</p>
            <p className="truncate text-xs font-medium text-zinc-200">HMAC-SHA256 verified</p>
          </div>
        </div>
      </div>

      {/* Main split: event list & JSON inspector */}
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-12">
        {/* Event List */}
        <div className="lg:col-span-5 flex flex-col gap-2">
          <p className="text-[11px] font-medium text-zinc-400 uppercase tracking-wider">Recent Captured Changes</p>
          <div className="space-y-2">
            {events.map((evt) => {
              const isSelected = selectedEvent.id === evt.id;
              const actionColors = {
                INSERT: "text-emerald-400 border-emerald-500/30 bg-emerald-950/30",
                UPDATE: "text-cyan-400 border-cyan-500/30 bg-cyan-950/30",
                DELETE: "text-amber-400 border-amber-500/30 bg-amber-950/30",
              };
              return (
                <button
                  key={evt.id}
                  onClick={() => setSelectedEvent(evt)}
                  className={`w-full text-left rounded-lg border p-3 transition-all ${
                    isSelected
                      ? "border-emerald-500/60 bg-emerald-950/30 shadow-[0_0_15px_rgba(16,185,129,0.15)]"
                      : "border-zinc-800/80 bg-[#09110c]/50 hover:border-emerald-900/60 hover:bg-[#0c1811]"
                  }`}
                >
                  <div className="flex items-center justify-between text-xs">
                    <span className={`font-pixel px-1.5 py-0.5 rounded border text-[10px] ${actionColors[evt.action]}`}>
                      {evt.action}
                    </span>
                    <span className="font-mono-code text-[11px] text-zinc-400">{evt.table}</span>
                    <span className="text-[10px] text-zinc-400 font-mono-code">{evt.lsn}</span>
                  </div>
                  <div className="mt-2 flex items-center justify-between text-[11px] text-zinc-400">
                    <span className="truncate">{evt.id}</span>
                    <span className="flex items-center gap-1 text-emerald-400 font-mono-code text-[10px]">
                      <Check className="h-3 w-3" /> 200 OK
                    </span>
                  </div>
                </button>
              );
            })}
          </div>
        </div>

        {/* Payload Inspector */}
        <div className="lg:col-span-7 flex flex-col">
          <div className="flex items-center justify-between rounded-t-lg border border-zinc-800/90 bg-[#08120d] px-3.5 py-2">
            <div className="flex items-center gap-2">
              <span className="font-pixel text-[11px] text-emerald-400">PAYLOAD</span>
              <span className="text-zinc-500">•</span>
              <span className="font-mono-code text-xs text-zinc-300">{selectedEvent.id}.json</span>
            </div>
            <button
              onClick={copyJson}
              className="flex items-center gap-1.5 text-xs text-zinc-400 transition-colors hover:text-emerald-300"
            >
              {copied ? <Check className="h-3.5 w-3.5 text-emerald-400" /> : <Copy className="h-3.5 w-3.5" />}
              <span>{copied ? "Copied" : "Copy"}</span>
            </button>
          </div>
          <div className="relative flex-1 overflow-x-auto rounded-b-lg border-x border-b border-zinc-800/90 bg-[#030604] p-3.5 font-mono-code text-xs text-emerald-400/90 leading-relaxed">
            <pre>{JSON.stringify(selectedEvent, null, 2)}</pre>
            <div className="mt-3 pt-3 border-t border-zinc-800/60 flex items-center justify-between text-[11px] text-zinc-400">
              <span className="flex items-center gap-1.5 text-zinc-400">
                <ShieldCheck className="h-3.5 w-3.5 text-emerald-400" />
                HMAC Header: <code className="text-emerald-300/80">{selectedEvent.signature}</code>
              </span>
              <span className="font-pixel text-[10px] text-emerald-500">SIGNED</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
