// Requests being sent from the request editor (NET-3). They live here, not
// in the editor, so a send keeps going when you open another page, the tray
// at the bottom lists them, and the editor shows the result when you return.
import { useSyncExternalStore } from "react";
import { post } from "./api";
import type { SendResult } from "./types";

export type Flight = {
  key: string;
  apiId: string;
  reqId: string;
  label: string;
  method: string;
  startedAt: number;
  finishedAt?: number;
  result?: SendResult;
  /** The editor for this request has shown the result. */
  seen?: boolean;
};

let flights: Flight[] = [];
const controllers = new Map<string, AbortController>();
const listeners = new Set<() => void>();
const emit = () => {
  for (const l of listeners) l();
};
const update = (key: string, f: (x: Flight) => Flight) => {
  flights = flights.map((x) => (x.key === key ? f(x) : x));
  emit();
};

export const flightKey = (apiId: string, reqId: string) => `${apiId}:${reqId || "new"}`;

/** send starts a request; a second send of the same request cancels the first. */
export async function send(
  meta: { apiId: string; reqId: string; label: string; method: string },
  body: unknown,
): Promise<SendResult> {
  const key = flightKey(meta.apiId, meta.reqId);
  controllers.get(key)?.abort();
  const c = new AbortController();
  controllers.set(key, c);
  flights = [...flights.filter((x) => x.key !== key), { ...meta, key, startedAt: Date.now() }];
  emit();
  let result: SendResult;
  try {
    result = await post<SendResult>("/api/send", body, c.signal);
  } catch (e) {
    const aborted = (e as Error).name === "AbortError";
    result = aborted
      ? { body: "", tokens: 0, error: "Canceled before a response came back.", code: "canceled" }
      : { body: "", tokens: 0, error: (e as Error).message };
  }
  if (controllers.get(key) === c) {
    controllers.delete(key);
    update(key, (x) => ({ ...x, finishedAt: Date.now(), result }));
  }
  return result;
}

export function cancel(key: string) {
  controllers.get(key)?.abort();
}

export function markSeen(key: string) {
  const f = flights.find((x) => x.key === key);
  if (f?.finishedAt && !f.seen) update(key, (x) => ({ ...x, seen: true }));
}

export function dismiss(key: string) {
  flights = flights.filter((x) => x.key !== key || !x.finishedAt);
  emit();
}

const subscribe = (l: () => void) => {
  listeners.add(l);
  return () => listeners.delete(l);
};

export const useFlights = () => useSyncExternalStore(subscribe, () => flights);
export const useFlight = (key: string) => useSyncExternalStore(subscribe, () => flights.find((x) => x.key === key));

/** For tests. */
export function resetFlights() {
  for (const c of controllers.values()) c.abort();
  controllers.clear();
  flights = [];
  emit();
}
