// Pickers for a login's steps (AUTH-2): which request signs in, and what to
// keep from its response. Requests that look like a sign-in come first, and
// the fields to keep are read from the request's last response, the way
// Postman and Insomnia offer response values when chaining requests.
import { Plus, Send, X } from "lucide-react";
import { useState } from "react";
import { flightKey, send, useFlight } from "../lib/inflight";
import type { LoginStep, Request, SendResult } from "../lib/types";
import { Button, Select } from "./ui";

const loginWords = /(log-?in|sign-?in|auth|token|session|oauth|sso|credential)/i;

/** loginScore ranks how much a request looks like a sign-in. */
export function loginScore(r: Pick<Request, "name" | "title" | "url" | "method" | "body">): number {
  let n = 0;
  if (loginWords.test(r.name) || loginWords.test(r.title ?? "")) n += 3;
  if (loginWords.test(r.url.replace(/\{\{[^}]*\}\}/g, ""))) n += 2;
  if (r.method === "POST") n += 1;
  if (/password|client_secret|grant_type/i.test(r.body ?? "")) n += 2;
  return n;
}

/** byLogin sorts requests so the likely sign-in requests come first. */
export function byLogin<T extends Pick<Request, "name" | "title" | "url" | "method" | "body">>(rs: T[]): T[] {
  return rs
    .map((r, i) => ({ r, i, s: loginScore(r) }))
    .sort((a, b) => b.s - a.s || a.i - b.i)
    .map((x) => x.r);
}

export type Pick2 = { value: string; sample: string };
export type Fields = { json: Pick2[]; header: Pick2[]; cookie: Pick2[] };

const short = (v: unknown) => {
  const s = typeof v === "string" ? v : JSON.stringify(v);
  return s.length > 40 ? `${s.slice(0, 39)}…` : s;
};

/** fieldsOf lists what a response offers to keep: JSON paths with sample
 * values, header names and cookie names. */
export function fieldsOf(res: { body?: unknown; headers?: Record<string, string[]> } | undefined): Fields {
  const out: Fields = { json: [], header: [], cookie: [] };
  if (!res) return out;
  let body = res.body;
  if (typeof body === "string") {
    try {
      body = JSON.parse(body);
    } catch {
      body = undefined;
    }
  }
  const walk = (v: unknown, path: string, depth: number) => {
    if (out.json.length >= 200) return;
    if (Array.isArray(v)) {
      if (v.length > 0 && depth < 5) walk(v[0], path ? `${path}.0` : "0", depth + 1);
      return;
    }
    if (v && typeof v === "object") {
      if (depth >= 5) return;
      for (const [k, x] of Object.entries(v)) walk(x, path ? `${path}.${k}` : k, depth + 1);
      return;
    }
    if (path) out.json.push({ value: path, sample: short(v) });
  };
  walk(body, "", 0);
  for (const [k, vs] of Object.entries(res.headers ?? {})) {
    if (k.toLowerCase() === "set-cookie") {
      for (const c of vs) {
        const [nv] = c.split(";");
        const i = nv.indexOf("=");
        if (i > 0) out.cookie.push({ value: nv.slice(0, i).trim(), sample: short(nv.slice(i + 1)) });
      }
    } else {
      out.header.push({ value: k, sample: short(vs.join(", ")) });
    }
  }
  return out;
}

/** lastResponse is the request's latest response in this session, or its
 * newest saved response, or its example. */
export function useLastResponse(apiId: string, r: Request | undefined) {
  const f = useFlight(flightKey(apiId, r?.id ?? ""));
  if (!r) return undefined;
  if (f?.result && !f.result.error) return { from: "last send", res: f.result as SendResult };
  const saved = [...(r.responses ?? [])].sort((a, b) => b.savedAt.localeCompare(a.savedAt))[0];
  if (saved) return { from: `saved response "${saved.name}"`, res: { body: saved.body } };
  if (r.example !== undefined && r.example !== null) return { from: "example", res: { body: r.example } };
  return undefined;
}

type Kind = "json" | "header" | "cookie" | "custom";
const kindOf = (from: string): Kind =>
  from.startsWith("json:")
    ? "json"
    : from.startsWith("header:")
      ? "header"
      : from.startsWith("cookie:")
        ? "cookie"
        : "custom";
const prefix: Record<Exclude<Kind, "custom">, string> = { json: "json:", header: "header:", cookie: "cookie:" };
const kindLabel: Record<Kind, string> = { json: "Body", header: "Header", cookie: "Cookie", custom: "Custom" };

/** CaptureRows edits a step's captures as Name, From and Field. */
export function CaptureRows({
  apiId,
  step,
  index,
  request,
  onChange,
  onBearer,
}: {
  apiId: string;
  step: LoginStep;
  index: number;
  request?: Request;
  onChange: (captures: NonNullable<LoginStep["captures"]>) => void;
  onBearer?: (name: string) => void;
}) {
  const caps = step.captures ?? [];
  const last = useLastResponse(apiId, request);
  const fields = fieldsOf(last?.res);
  const [sending, setSending] = useState(false);
  const setCap = (i: number, c: { name: string; from: string }) => onChange(caps.map((x, j) => (j === i ? c : x)));
  const tail = (p: string) =>
    p
      .split(".")
      .filter((s) => !/^\d+$/.test(s))
      .pop() ?? p;
  const n = index + 1;
  return (
    <div className="captures">
      <div className="captures-head">
        <span>Keep from the response</span>
        {request &&
          (last ? (
            <span className="muted text-[12px]">Fields from the {last.from}</span>
          ) : (
            <span className="muted text-[12px]">
              Send the request once to pick fields from its response.{" "}
              <Button
                size="sm"
                kind="ghost"
                icon={<Send size={12} />}
                loading={sending}
                onClick={async () => {
                  setSending(true);
                  await send(
                    { apiId, reqId: request.id, label: request.name, method: request.method },
                    { apiId, requestId: request.id },
                  );
                  setSending(false);
                }}
              >
                Send {request.name}
              </Button>
            </span>
          ))}
      </div>
      {caps.map((c, i) => {
        const kind = kindOf(c.from);
        const field = kind === "custom" ? c.from : c.from.slice(prefix[kind].length);
        const opts = kind === "custom" ? [] : fields[kind];
        const options = [
          ...(field && !opts.some((o) => o.value === field) ? [{ value: field, sample: "" }] : []),
          ...opts,
        ].map((o) => ({ value: o.value, label: <span className="mono">{o.value}</span>, hint: o.sample || undefined }));
        return (
          <div key={i} className="capture-row">
            <input
              className="input mono"
              aria-label={`Step ${n} capture ${i + 1} name`}
              placeholder="access_token"
              value={c.name}
              onChange={(e) => setCap(i, { ...c, name: e.target.value })}
            />
            <Select<Kind>
              label={`Step ${n} capture ${i + 1} from`}
              value={kind}
              onChange={(k) => setCap(i, { ...c, from: k === "custom" ? "" : prefix[k] })}
              options={(["json", "header", "cookie", "custom"] as Kind[]).map((k) => ({
                value: k,
                label: kindLabel[k],
                hint: k === "custom" ? "Any expression, e.g. regex:..." : undefined,
              }))}
            />
            {kind === "custom" || options.length === 0 ? (
              <input
                className="input mono"
                aria-label={`Step ${n} capture ${i + 1} field`}
                placeholder={
                  kind === "custom"
                    ? "regex:token=([a-z0-9]+)"
                    : kind === "json"
                      ? "access_token or data.token"
                      : kind === "header"
                        ? "X-Auth-Token"
                        : "sessionid"
                }
                value={field}
                onChange={(e) =>
                  setCap(i, { ...c, from: kind === "custom" ? e.target.value : prefix[kind] + e.target.value })
                }
              />
            ) : (
              <Select
                label={`Step ${n} capture ${i + 1} field`}
                placeholder="Choose a field"
                value={field}
                onChange={(v) =>
                  setCap(i, { name: c.name || tail(v), from: prefix[kind as Exclude<Kind, "custom">] + v })
                }
                options={options}
              />
            )}
            <div className="flex gap-1">
              {onBearer && c.name && (
                <Button
                  size="sm"
                  kind="ghost"
                  title="Send it as Authorization: Bearer"
                  onClick={() => onBearer(c.name)}
                >
                  Use as bearer
                </Button>
              )}
              <Button
                size="sm"
                kind="ghost"
                icon={<X size={13} />}
                aria-label={`Remove capture ${c.name || i + 1}`}
                onClick={() => onChange(caps.filter((_, j) => j !== i))}
              />
            </div>
          </div>
        );
      })}
      <div>
        <Button
          size="sm"
          kind="ghost"
          icon={<Plus size={13} />}
          onClick={() => onChange([...caps, { name: "", from: "json:" }])}
        >
          Add a value to keep
        </Button>
      </div>
    </div>
  );
}
