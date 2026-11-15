import { NextResponse } from "next/server";
import { parseEvent, streamName, withRedis } from "@/lib/redis";

export const dynamic = "force-dynamic";

export async function GET() {
  try {
    const events = await withRedis(async (client) => {
      const entries = await client.xRevRange(streamName, "+", "-", { COUNT: 100 });
      return entries.map((entry) => parseEvent(entry.id, entry.message));
    });
    return NextResponse.json({ events });
  } catch {
    return NextResponse.json({ events: [], error: "Redis is unavailable. Start local Redis to view events." }, { status: 503 });
  }
}
