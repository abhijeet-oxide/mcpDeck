// Unsaved edits to requests, kept while you look at other requests or pages
// (NET-3), like Postman's unsaved tabs. They last until saved, discarded or
// the page reloads.
import { useSyncExternalStore } from "react";
import type { Request } from "./types";

let drafts = new Map<string, Request>();
const listeners = new Set<() => void>();
const emit = () => {
  for (const l of listeners) l();
};

export const getDraft = (key: string) => drafts.get(key);

export function setDraft(key: string, r: Request | undefined) {
  if (r === undefined ? !drafts.has(key) : drafts.get(key) === r) return;
  drafts = new Map(drafts);
  if (r === undefined) drafts.delete(key);
  else drafts.set(key, r);
  emit();
}

const subscribe = (l: () => void) => {
  listeners.add(l);
  return () => listeners.delete(l);
};
export const useDrafts = () => useSyncExternalStore(subscribe, () => drafts);

export function resetDrafts() {
  drafts = new Map();
  emit();
}
