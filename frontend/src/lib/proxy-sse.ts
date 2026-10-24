// next.config.ts rewrites don't stream SSE properly - only the first chunk
// gets through. This is a workaround to proxy the request to the backend.
const BACKEND_ORIGIN = "http://localhost:8888";

export async function proxySSE(request: Request, path: string) {
  const upstream = await fetch(`${BACKEND_ORIGIN}${path}`, {
    headers: {
      cookie: request.headers.get("cookie") ?? "",
      "x-api-key": request.headers.get("x-api-key") ?? "",
    },
    signal: request.signal,
  });

  return new Response(upstream.body, {
    status: upstream.status,
    headers: {
      "Content-Type": upstream.headers.get("Content-Type") ?? "text/event-stream",
      "Cache-Control": "no-cache",
      Connection: "keep-alive",
    },
  });
}
