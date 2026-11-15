import { createClient } from "redis";
import { parseEvent, streamName } from "@/lib/redis";

export const dynamic = "force-dynamic";
export const runtime = "nodejs";

export async function GET(request: Request) {
  const url = new URL(request.url);
  let lastID = request.headers.get("last-event-id") ?? url.searchParams.get("after") ?? "$";
  const encoder = new TextEncoder();
  let active = true;
  let client: ReturnType<typeof createClient> | undefined;
  let heartbeat: ReturnType<typeof setInterval> | undefined;

  const close = async () => {
    active = false;
    if (heartbeat) clearInterval(heartbeat);
    if (client?.isOpen) await client.quit();
  };

  const body = new ReadableStream({
    start(controller) {
      const send = (message: string) => controller.enqueue(encoder.encode(message));
      const abort = () => void close();
      request.signal.addEventListener("abort", abort, { once: true });

      void (async () => {
        try {
          client = createClient({ url: process.env.REDIS_URL ?? "redis://localhost:6379" });
          await client.connect();
          send("retry: 1000\n\n");
          heartbeat = setInterval(() => {
            if (active) send("event: heartbeat\ndata: {}\n\n");
          }, 15000);

          while (active) {
            const streams = await client.xRead([{ key: streamName, id: lastID }], { COUNT: 20, BLOCK: 1000 });
            if (!streams) continue;
            for (const stream of streams) {
              for (const entry of stream.messages) {
                lastID = entry.id;
                const event = parseEvent(entry.id, entry.message);
                send(`id: ${entry.id}\nevent: cdc\ndata: ${JSON.stringify({ stream_id: entry.id, event })}\n\n`);
              }
            }
          }
        } catch {
          if (active) send("event: error\ndata: {\"error\":\"Redis stream unavailable\"}\n\n");
        } finally {
          await close();
          request.signal.removeEventListener("abort", abort);
          controller.close();
        }
      })();
    },
    cancel() {
      return close();
    },
  });

  return new Response(body, {
    headers: {
      "Cache-Control": "no-cache, no-transform",
      Connection: "keep-alive",
      "Content-Type": "text/event-stream",
    },
  });
}
