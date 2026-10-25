import { proxySSE } from "@/lib/proxy-sse";

export const dynamic = "force-dynamic";

export async function GET(request: Request) {
  return proxySSE(request, "/api/mediaserver/library/refresh/stream");
}
