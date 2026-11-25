import { Check, Copy, X } from "lucide-react";
import {
  type ButtonHTMLAttributes,
  createContext,
  type ReactNode,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
} from "react";

type BtnKind = "default" | "ink" | "blue" | "bad" | "ghost";
export function Button({
  kind = "default",
  size,
  icon,
  className = "",
  children,
  ...rest
}: ButtonHTMLAttributes<HTMLButtonElement> & { kind?: BtnKind; size?: "sm"; icon?: ReactNode }) {
  const cls = [
    "btn",
    kind !== "default" && `btn-${kind}`,
    size === "sm" && "btn-sm",
    !children && "btn-icon",
    className,
  ]
    .filter(Boolean)
    .join(" ");
  return (
    <button type="button" className={cls} {...rest}>
      {icon}
      {children}
    </button>
  );
}

export function Chip({
  tone,
  children,
  title,
}: {
  tone?: "ok" | "info" | "warn" | "bad";
  children: ReactNode;
  title?: string;
}) {
  return (
    <span className={`chip${tone ? ` chip-${tone}` : ""}`} title={title}>
      {children}
    </span>
  );
}

export function MethodChip({ method }: { method: string }) {
  const m = method.toUpperCase();
  const cls = ["GET", "POST", "PUT", "PATCH", "DELETE"].includes(m) ? m : "OTHER";
  return <span className={`method m-${cls}`}>{m}</span>;
}

export function AccessChip({ access, flag }: { access: string; flag?: boolean }) {
  if (flag) return <Chip tone="warn">check access</Chip>;
  return access === "read" ? <Chip tone="info">read</Chip> : <Chip tone="warn">writes</Chip>;
}

export function StatusChip({ status }: { status: string }) {
  const tone = {
    live: "ok",
    pending: "warn",
    rejected: "bad",
    retired: undefined,
    ok: "ok",
    error: "bad",
    blocked: "bad",
  }[status] as "ok" | "warn" | "bad" | undefined;
  const label = status === "pending" ? "needs review" : status;
  return <Chip tone={tone}>{label}</Chip>;
}

export function Panel({
  title,
  icon,
  right,
  children,
  className = "",
  bodyClass,
}: {
  title?: ReactNode;
  icon?: ReactNode;
  right?: ReactNode;
  children: ReactNode;
  className?: string;
  bodyClass?: string;
}) {
  return (
    <section className={`panel ${className}`}>
      {title !== undefined && (
        <div className="panel-head">
          {icon}
          <h2>{title}</h2>
          {right && <div className="right">{right}</div>}
        </div>
      )}
      <div className={bodyClass ?? ""}>{children}</div>
    </section>
  );
}

export function PageHead({
  title,
  sub,
  crumbs,
  actions,
  mono,
}: {
  title: ReactNode;
  sub?: ReactNode;
  crumbs?: ReactNode;
  actions?: ReactNode;
  mono?: boolean;
}) {
  return (
    <div className="page-head">
      <div className="min-w-0">
        {crumbs && <div className="crumbs">{crumbs}</div>}
        <h1 className={mono ? "mono" : ""} style={mono ? { fontSize: 19 } : undefined}>
          {title}
        </h1>
        {sub && <div className="sub">{sub}</div>}
      </div>
      {actions && <div className="actions">{actions}</div>}
    </div>
  );
}

export function Tabs<T extends string>({
  tabs,
  value,
  onChange,
}: {
  tabs: { id: T; label: ReactNode; n?: number }[];
  value: T;
  onChange: (t: T) => void;
}) {
  return (
    <div className="tabs" role="tablist">
      {tabs.map((t) => (
        <button key={t.id} type="button" role="tab" aria-selected={value === t.id} onClick={() => onChange(t.id)}>
          {t.label}
          {t.n !== undefined && t.n > 0 && <span className="n">{t.n}</span>}
        </button>
      ))}
    </div>
  );
}

export function Seg<T extends string>({
  options,
  value,
  onChange,
  label,
}: {
  options: { id: T; label: ReactNode }[];
  value: T;
  onChange: (v: T) => void;
  label: string;
}) {
  return (
    <div className="seg" role="group" aria-label={label}>
      {options.map((o) => (
        <button key={o.id} type="button" aria-pressed={value === o.id} onClick={() => onChange(o.id)}>
          {o.label}
        </button>
      ))}
    </div>
  );
}

export function Dialog({
  title,
  onClose,
  children,
  footer,
  wide,
}: {
  title: ReactNode;
  onClose: () => void;
  children: ReactNode;
  footer?: ReactNode;
  wide?: boolean;
}) {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const k = (e: KeyboardEvent) => e.key === "Escape" && onClose();
    window.addEventListener("keydown", k);
    const first = ref.current?.querySelector<HTMLElement>("input, select, textarea, button:not([aria-label='Close'])");
    first?.focus();
    return () => window.removeEventListener("keydown", k);
  }, []);
  return (
    <div className="overlay" onMouseDown={(e) => e.target === e.currentTarget && onClose()}>
      <div className={`dialog${wide ? " dialog-wide" : ""}`} role="dialog" aria-modal="true" ref={ref}>
        <div className="dialog-head">
          <h2>{title}</h2>
          <Button
            kind="ghost"
            size="sm"
            className="ml-auto"
            aria-label="Close"
            onClick={onClose}
            icon={<X size={15} />}
          />
        </div>
        <div className="dialog-body">{children}</div>
        {footer && <div className="dialog-foot">{footer}</div>}
      </div>
    </div>
  );
}

export function Drawer({ title, onClose, children }: { title: ReactNode; onClose: () => void; children: ReactNode }) {
  useEffect(() => {
    const k = (e: KeyboardEvent) => e.key === "Escape" && onClose();
    window.addEventListener("keydown", k);
    return () => window.removeEventListener("keydown", k);
  }, []);
  return (
    <aside className="drawer" aria-label="Details">
      <div className="dialog-head">
        <h2 className="min-w-0 truncate">{title}</h2>
        <Button
          kind="ghost"
          size="sm"
          className="ml-auto"
          aria-label="Close"
          onClick={onClose}
          icon={<X size={15} />}
        />
      </div>
      <div className="dialog-body flex-1">{children}</div>
    </aside>
  );
}

export function Empty({ title, children }: { title: string; children?: ReactNode }) {
  return (
    <div className="empty">
      <strong>{title}</strong>
      {children}
    </div>
  );
}

export function Field({ label, hint, children }: { label: string; hint?: ReactNode; children: ReactNode }) {
  return (
    <label className="field">
      <span>{label}</span>
      {children}
      {hint && <div className="hint">{hint}</div>}
    </label>
  );
}

// Toasts
type Toast = { id: number; text: string; bad?: boolean };
const ToastCtx = createContext<(text: string, bad?: boolean) => void>(() => {});
export function ToastProvider({ children }: { children: ReactNode }) {
  const [list, setList] = useState<Toast[]>([]);
  const push = useCallback((text: string, bad?: boolean) => {
    const id = Date.now() + Math.random();
    setList((l) => [...l, { id, text, bad }]);
    setTimeout(() => setList((l) => l.filter((t) => t.id !== id)), bad ? 6000 : 3000);
  }, []);
  return (
    <ToastCtx.Provider value={push}>
      {children}
      <div className="toast-wrap" role="status" aria-live="polite">
        {list.map((t) => (
          <div key={t.id} className={`toast${t.bad ? " bad" : ""}`}>
            {t.text}
          </div>
        ))}
      </div>
    </ToastCtx.Provider>
  );
}
export const useToast = () => useContext(ToastCtx);

export function useCopy() {
  const toast = useToast();
  return (text: string, what = "Copied") => {
    navigator.clipboard?.writeText(text).then(
      () => toast(what),
      () => toast("Copy failed; select the text instead", true),
    );
  };
}

export function CopyButton({ text, label = "Copy", what }: { text: string; label?: string; what?: string }) {
  const copy = useCopy();
  const [done, setDone] = useState(false);
  return (
    <Button
      size="sm"
      icon={done ? <Check size={13} /> : <Copy size={13} />}
      onClick={() => {
        copy(text, what);
        setDone(true);
        setTimeout(() => setDone(false), 1500);
      }}
    >
      {label}
    </Button>
  );
}

export function CopyField({ value, label }: { value: string; label?: string }) {
  return (
    <div className="copy-field" title={value}>
      <code>{value}</code>
      <CopyButton text={value} what={label ? `${label} copied` : "Copied"} />
    </div>
  );
}

export function ErrorBox({ error }: { error: unknown }) {
  if (!error) return null;
  return <div className="banner bad">{error instanceof Error ? error.message : String(error)}</div>;
}

export function Loading() {
  return <div className="empty">Loading…</div>;
}

export function Toggle({
  checked,
  onChange,
  label,
}: {
  checked: boolean;
  onChange: (v: boolean) => void;
  label: string;
}) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={label}
      onClick={() => onChange(!checked)}
      style={{
        width: 30,
        height: 18,
        borderRadius: 999,
        border: 0,
        padding: 2,
        background: checked ? "var(--accent)" : "var(--line-strong)",
        cursor: "pointer",
        flex: "none",
      }}
    >
      <span
        style={{
          display: "block",
          width: 14,
          height: 14,
          borderRadius: "50%",
          background: "#fff",
          transform: checked ? "translateX(12px)" : "none",
          transition: "transform .12s",
        }}
      />
    </button>
  );
}
