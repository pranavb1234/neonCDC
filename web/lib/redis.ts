import { createClient } from "redis";
import type { CdcEvent } from "@/lib/types";

const streamName = "cdc:events";

export async function withRedis<T>(work: (client: ReturnType<typeof createClient>) => Promise<T>) {
  const client = createClient({ url: process.env.REDIS_URL ?? "redis://localhost:6379" });
  await client.connect();
  try {
    return await work(client);
  } finally {
    await client.quit();
  }
}

export function parseEvent(streamID: string, message: Record<string, string>): CdcEvent {
  const payload = JSON.parse(message.payload ?? "{}") as Omit<CdcEvent, "stream_id">;
  return { ...payload, stream_id: streamID };
}

export { streamName };
