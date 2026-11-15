import { NextResponse } from "next/server";
import { streamName, withRedis } from "@/lib/redis";

export const dynamic = "force-dynamic";

export async function GET() {
  try {
    const data = await withRedis(async (client) => {
      const [eventCount, groups] = await Promise.all([client.xLen(streamName), client.xInfoGroups(streamName)]);
      const delivery = groups.find((group) => group.name === "delivery");
      return { eventCount, pending: delivery?.pending ?? 0, consumers: delivery?.consumers ?? 0 };
    });
    return NextResponse.json({ ...data, webhookUrl: process.env.WEBHOOK_URL ?? "Not configured" });
  } catch {
    return NextResponse.json({ eventCount: 0, pending: 0, consumers: 0, webhookUrl: process.env.WEBHOOK_URL ?? "Not configured", error: "Redis is unavailable" }, { status: 503 });
  }
}
