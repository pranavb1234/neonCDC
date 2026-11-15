"use client";

import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";

export function WebhookSettings() {
  const [url, setURL] = useState("Loading…"); const [result, setResult] = useState<string>(); const [sending, setSending] = useState(false);
  useEffect(() => { fetch("/api/overview").then((response) => response.json()).then((data) => setURL(data.webhookUrl ?? "Not configured")).catch(() => setURL("Not configured")); }, []);
  async function sendTest() { setSending(true); setResult(undefined); const response = await fetch("/api/webhook/test", { method: "POST" }); const data = await response.json(); setResult(response.ok ? `Delivered test event ${data.eventId}` : data.error); setSending(false); }
  return <><div className="mb-8"><p className="text-sm text-emerald-400">Local delivery target</p><h1 className="mt-1 text-3xl font-bold">Webhook Settings</h1></div><Card><p className="text-sm text-zinc-400">Configured webhook URL</p><p className="mt-2 break-all font-mono text-sm">{url}</p><div className="mt-6"><Button onClick={sendTest} disabled={sending}>{sending ? "Sending…" : "Send test event"}</Button>{result ? <p className="mt-3 text-sm text-zinc-300">{result}</p> : null}</div><p className="mt-6 text-sm text-zinc-500">Test events are HMAC-signed using your local `WEBHOOK_SECRET` and do not enter the Redis stream.</p></Card></>;
}
