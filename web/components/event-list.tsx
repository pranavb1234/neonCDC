"use client";

import { useEffect, useMemo, useState } from "react";
import { ChevronDown, ChevronRight } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import type { CdcEvent } from "@/lib/types";

export function EventList() {
  const [events, setEvents] = useState<CdcEvent[]>([]); const [table, setTable] = useState("all"); const [operation, setOperation] = useState("all"); const [expanded, setExpanded] = useState<string>(); const [error, setError] = useState<string>();
  useEffect(() => {
    let source: EventSource | undefined;
    let disposed = false;
    fetch("/api/events").then(async (response) => {
      const data = await response.json();
      const initialEvents = data.events ?? [];
      if (disposed) return;
      setEvents(initialEvents);
      setError(data.error);

      const newestID = initialEvents[0]?.stream_id ?? "0-0";
      source = new EventSource(`/api/events/stream?after=${encodeURIComponent(newestID)}`);
      source.addEventListener("cdc", (message) => {
        const incoming = JSON.parse((message as MessageEvent<string>).data) as { event: CdcEvent };
        setEvents((current) => [incoming.event, ...current.filter((event) => event.stream_id !== incoming.event.stream_id)].slice(0, 100));
        setError(undefined);
      });
      source.addEventListener("error", () => setError("Redis stream is temporarily unavailable. Reconnecting…"));
    }).catch(() => setError("Could not load events"));

    return () => { disposed = true; source?.close(); };
  }, []);
  const tables = useMemo(() => [...new Set(events.map((event) => event.table))], [events]);
  const filtered = events.filter((event) => (table === "all" || event.table === table) && (operation === "all" || event.operation === operation));
  return <><div className="mb-8"><p className="text-sm text-emerald-400">Live updates from Redis Streams</p><h1 className="mt-1 text-3xl font-bold">Live Events</h1></div><div className="mb-5 flex flex-wrap gap-3"><select value={table} onChange={(event) => setTable(event.target.value)} className="rounded-md border border-zinc-700 bg-zinc-900 px-3 py-2 text-sm"><option value="all">All tables</option>{tables.map((name) => <option key={name}>{name}</option>)}</select><select value={operation} onChange={(event) => setOperation(event.target.value)} className="rounded-md border border-zinc-700 bg-zinc-900 px-3 py-2 text-sm"><option value="all">All operations</option><option value="insert">Insert</option><option value="update">Update</option><option value="delete">Delete</option></select></div>{error ? <Card><p className="text-amber-200">{error}</p></Card> : null}{!error && filtered.length === 0 ? <Card><p className="font-medium">No events yet</p><p className="mt-1 text-sm text-zinc-400">Start the connector, then insert, update, or delete a row in Neon.</p></Card> : <div className="space-y-2">{filtered.map((event) => <Card key={event.stream_id} className="p-0"><button onClick={() => setExpanded(expanded === event.stream_id ? undefined : event.stream_id)} className="grid w-full grid-cols-[24px_1fr_auto] items-center gap-3 p-4 text-left"><span>{expanded === event.stream_id ? <ChevronDown size={16} /> : <ChevronRight size={16} />}</span><div><div className="flex items-center gap-2"><span className="font-medium">{event.table}</span><Badge variant={event.operation}>{event.operation}</Badge></div><p className="mt-1 truncate font-mono text-xs text-zinc-500">{event.id}</p></div><time className="text-xs text-zinc-400">{new Date(event.created_at).toLocaleString()}</time></button>{expanded === event.stream_id ? <pre className="overflow-x-auto border-t border-zinc-800 bg-zinc-950 p-4 text-xs leading-6 text-emerald-100">{JSON.stringify(event, null, 2)}</pre> : null}</Card>)}</div>}</>;
}
