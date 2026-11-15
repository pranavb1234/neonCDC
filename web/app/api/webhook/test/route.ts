import { randomUUID } from "crypto";
import { NextResponse } from "next/server";
import { signWebhook } from "@/lib/webhook";

export const runtime = "nodejs";

export async function POST() {
  const webhookURL = process.env.WEBHOOK_URL;
  const webhookSecret = process.env.WEBHOOK_SECRET;
  if (!webhookURL || !webhookSecret) {
    return NextResponse.json({ error: "WEBHOOK_URL and WEBHOOK_SECRET are required" }, { status: 500 });
  }

  const event = { id: `test-${randomUUID()}`, table: "dashboard", operation: "insert", commit_lsn: "test", data: { source: "dashboard" }, created_at: new Date().toISOString() };
  const body = JSON.stringify(event);
  const timestamp = Math.floor(Date.now() / 1000).toString();
  try {
    const response = await fetch(webhookURL, { method: "POST", headers: { "Content-Type": "application/json", "X-CDC-Event-Id": event.id, "X-CDC-Timestamp": timestamp, "X-CDC-Signature": `v1=${signWebhook(webhookSecret, timestamp, body)}` }, body });
    if (!response.ok) return NextResponse.json({ error: `Webhook returned HTTP ${response.status}` }, { status: 502 });
    return NextResponse.json({ eventId: event.id });
  } catch {
    return NextResponse.json({ error: "Could not reach the configured webhook" }, { status: 502 });
  }
}
