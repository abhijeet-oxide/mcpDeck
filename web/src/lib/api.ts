// The one place the UI talks to the server. Every call is a route in
// internal/server (RUN-6).
export class ApiError extends Error {
  status: number;
  code: string;
  constructor(status: number, code: string, message: string) {
    super(message);
    this.status = status;
    this.code = code;
  }
}

export async function api<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(path, {
    method,
    headers: body === undefined ? undefined : { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
    credentials: "same-origin",
  });
  if (res.status === 204) return undefined as T;
  const text = await res.text();
  let data: any = text;
  try {
    data = text ? JSON.parse(text) : undefined;
  } catch {
    /* plain text */
  }
  if (!res.ok) {
    const msg = (data && typeof data === "object" && data.error) || `mcpDeck returned ${res.status}`;
    throw new ApiError(res.status, data?.code || "error", msg);
  }
  return data as T;
}

export const get = <T>(path: string) => api<T>("GET", path);
export const post = <T>(path: string, body?: unknown) => api<T>("POST", path, body ?? {});
export const put = <T>(path: string, body?: unknown) => api<T>("PUT", path, body ?? {});
export const patch = <T>(path: string, body?: unknown) => api<T>("PATCH", path, body ?? {});
export const del = <T>(path: string) => api<T>("DELETE", path);
