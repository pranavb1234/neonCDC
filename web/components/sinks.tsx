"use client";

import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";

type Pipeline = { id: number; name: string; status: string };
type Attempt = { event_id: string; status: "success" | "failed"; http_status?: number; error?: string; attempted_at: string };
type Sink = { id: number; name: string; pipeline_name: string; webhook_url: string; enabled: boolean; health: "healthy" | "degraded" | "failing" | "paused" | "unknown"; last_successful_at?: string; last_failed_at?: string; last_error?: string; recent_attempts: Attempt[] };

const healthStyle: Record<Sink["health"], string> = { healthy: "text-emerald-400", degraded: "text-amber-300", failing: "text-rose-300", paused: "text-zinc-400", unknown: "text-zinc-400" };
const displayTime = (value?: string) => value ? new Date(value).toLocaleString() : "—";

export function Sinks() {
  const [sinks, setSinks] = useState<Sink[]>([]);
  const [pipelines, setPipelines] = useState<Pipeline[]>([]);
  const [creating, setCreating] = useState(false);
  const [review, setReview] = useState(false);
  const [name, setName] = useState("");
  const [pipelineID, setPipelineID] = useState("");
  const [url, setURL] = useState("");
  const [secret, setSecret] = useState("");
  const [enabled, setEnabled] = useState(true);
  const [message, setMessage] = useState("");

  const load = async () => {
    const response = await fetch("/api/sinks");
    const data = await response.json();
    if (!response.ok) { setMessage(data.error ?? "Could not load sinks."); return; }
    setSinks(data.sinks ?? []); setPipelines(data.pipelines ?? []);
  };
  useEffect(() => { void load(); }, []);

  const create = async () => {
    const response = await fetch("/api/sinks", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ name, pipeline_id: Number(pipelineID), webhook_url: url, signing_secret: secret, enabled }) });
    const data = await response.json();
    if (!response.ok) { setMessage(data.error); return; }
    setMessage(data.generated_secret ? `Save this generated signing secret now: ${data.generated_secret}` : data.message);
    setCreating(false); setReview(false); setSecret(""); await load();
  };
  const toggle = async (sink: Sink) => {
    const response = await fetch(`/api/sinks/${sink.id}/enabled`, { method: "PATCH", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ enabled: !sink.enabled }) });
    if (!response.ok) { setMessage((await response.json()).error); return; }
    await load();
  };
  const sendTest = async (sink: Sink) => {
    const response = await fetch(`/api/sinks/${sink.id}/test`, { method: "POST" });
    const data = await response.json();
    setMessage(response.ok ? `Test sent to ${sink.name}.` : data.error ?? "Webhook test failed.");
    await load();
  };

  const selectedPipeline = pipelines.find((pipeline) => pipeline.id === Number(pipelineID));
  return <>
    <div className="mb-8 flex justify-between"><div><p className="text-sm text-emerald-400">Event destinations</p><h1 className="text-3xl font-bold">Sinks</h1><p className="mt-1 text-sm text-zinc-400">Health is based only on observed webhook attempts.</p></div><Button onClick={() => { setCreating(true); setMessage(""); }}>Create Sink</Button></div>
    {message ? <Card className="mb-4"><p className="text-sm text-zinc-200">{message}</p></Card> : null}
    {creating ? <Card className="mb-4"><h2 className="font-semibold">Create a Webhook sink</h2>
      {!review ? <div className="mt-4 space-y-3"><p className="text-sm text-zinc-400">Webhook is the only sink type in this first release.</p><input placeholder="Sink name" value={name} onChange={(e) => setName(e.target.value)} className="w-full rounded border border-zinc-700 bg-zinc-950 p-2" /><select value={pipelineID} onChange={(e) => setPipelineID(e.target.value)} className="w-full rounded border border-zinc-700 bg-zinc-950 p-2"><option value="">Choose source pipeline</option>{pipelines.map((pipeline) => <option key={pipeline.id} value={pipeline.id}>{pipeline.name} ({pipeline.status})</option>)}</select><input placeholder="https://receiver.example/webhook" value={url} onChange={(e) => setURL(e.target.value)} className="w-full rounded border border-zinc-700 bg-zinc-950 p-2" /><input placeholder="Signing secret (optional; generated if blank)" value={secret} onChange={(e) => setSecret(e.target.value)} className="w-full rounded border border-zinc-700 bg-zinc-950 p-2" /><label className="text-sm"><input type="checkbox" checked={enabled} onChange={(e) => setEnabled(e.target.checked)} /> Enabled after creation</label><div className="flex gap-2"><Button disabled={!name || !pipelineID || !url} onClick={() => setReview(true)}>Continue to review</Button><Button variant="secondary" onClick={() => setCreating(false)}>Cancel</Button></div></div> : <div className="mt-4 space-y-3"><p><b>Sink type:</b> Webhook</p><p><b>Name:</b> {name}</p><p><b>Pipeline:</b> {selectedPipeline?.name}</p><p><b>Webhook:</b> {url}</p><p><b>Initial state:</b> {enabled ? "Enabled" : "Disabled"}</p><p className="text-sm text-zinc-400">The saved secret will never be shown again. Redirects and loopback, private, link-local, and cloud-metadata addresses are blocked.</p><Button onClick={() => void create()}>Create Sink</Button> <Button variant="secondary" onClick={() => setReview(false)}>Back</Button></div>}
    </Card> : null}
    {sinks.length === 0 ? <Card>No sinks created. A saved configuration is shown as Unknown until an actual delivery is observed.</Card> : <div className="space-y-3">{sinks.map((sink) => <Card key={sink.id}>
      <div className="flex flex-wrap items-start justify-between gap-3"><div><h2 className="font-semibold">{sink.name} <span className={healthStyle[sink.health]}>· {sink.health}</span></h2><p className="text-sm text-zinc-400">Pipeline: {sink.pipeline_name} · {sink.enabled ? "enabled" : "disabled"}</p><p className="text-sm text-zinc-400">{sink.webhook_url}</p></div><div className="flex gap-2"><Button variant="secondary" onClick={() => void sendTest(sink)}>Send Test Event</Button><Button variant="secondary" onClick={() => void toggle(sink)}>{sink.enabled ? "Disable" : "Enable"}</Button></div></div>
      <div className="mt-3 grid gap-2 text-sm text-zinc-300 md:grid-cols-2"><p>Last success: {displayTime(sink.last_successful_at)}</p><p>Last failure: {displayTime(sink.last_failed_at)}</p>{sink.last_error ? <p className="text-rose-200 md:col-span-2">Last error: {sink.last_error}</p> : null}</div>
      <div className="mt-4"><h3 className="text-sm font-medium">Recent delivery attempts</h3>{sink.recent_attempts.length === 0 ? <p className="mt-1 text-sm text-zinc-400">No delivery attempts yet.</p> : <div className="mt-2 space-y-1 text-sm">{sink.recent_attempts.map((attempt, index) => <div className="rounded bg-zinc-950 p-2" key={`${attempt.event_id}-${index}`}><span className={attempt.status === "success" ? "text-emerald-400" : "text-rose-300"}>{attempt.status}</span> · {displayTime(attempt.attempted_at)} · event {attempt.event_id}{attempt.http_status ? ` · HTTP ${attempt.http_status}` : ""}{attempt.error ? ` · ${attempt.error}` : ""}</div>)}</div>}</div>
    </Card>)}</div>}
  </>;
}
