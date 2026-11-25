// Name rules shared by the forms, mirroring internal/deck.

/** toolkitSlug mirrors deck.Slug: lowercase letters, digits and dashes. */
export function toolkitSlug(s: string): string {
  return s
    .trim()
    .toLowerCase()
    .replace(/[ _]/g, "-")
    .replace(/[^a-z0-9-]/g, "")
    .replace(/^-+|-+$/g, "");
}

export function toolkitNameError(raw: string, taken: string[] = []): string {
  const s = toolkitSlug(raw);
  if (!raw.trim()) return "";
  if (s.length < 2) return "Use at least two letters or digits.";
  if (taken.includes(s)) return `A toolkit named ${s} already exists.`;
  return "";
}

/** appName turns a title like "Acme CRM" into a code name like acmeCrm. */
export function appName(s: string): string {
  const words = s
    .trim()
    .replace(/[^A-Za-z0-9]+/g, " ")
    .trim()
    .split(" ")
    .filter(Boolean);
  const out = words
    .map((w, i) => (i === 0 ? w.charAt(0).toLowerCase() + w.slice(1) : w.charAt(0).toUpperCase() + w.slice(1)))
    .join("")
    .replace(/^[0-9]+/, "");
  return out.slice(0, 32);
}

/** appNameError mirrors deck's apiNameRe: ^[a-z][a-zA-Z0-9]{1,31}$ */
export function appNameError(s: string, taken: string[] = []): string {
  if (!s) return "";
  if (!/^[a-z]/.test(s)) return "Start with a lowercase letter.";
  if (/[^a-zA-Z0-9]/.test(s)) return "Letters and digits only, like acmeCrm.";
  if (s.length < 2) return "Use at least two characters.";
  if (s.length > 32) return "Use 32 characters or fewer.";
  if (taken.includes(s)) return `An application named ${s} already exists.`;
  return "";
}

/** detectImport guesses what a dropped file is, so people don't have to say. */
export type ImportKind = "postman" | "postman-env" | "spec" | "curl" | "unknown";
export function detectImport(text: string): ImportKind {
  const t = text.trim();
  if (/^curl\s/.test(t)) return "curl";
  try {
    const o = JSON.parse(t);
    if (o && typeof o === "object") {
      if (o.openapi || o.swagger) return "spec";
      if (Array.isArray(o.item) || o.info?._postman_id || String(o.info?.schema ?? "").includes("postman"))
        return "postman";
      if (Array.isArray(o.values) && (o._postman_variable_scope || "name" in o)) return "postman-env";
    }
    return "unknown";
  } catch {
    if (/^(openapi|swagger)\s*:/m.test(t)) return "spec";
    return "unknown";
  }
}
