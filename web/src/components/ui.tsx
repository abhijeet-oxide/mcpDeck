import { Check, CheckCircle2, CircleAlert, Copy, Loader2, MoreHorizontal, X } from "lucide-react";
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
  loading,
  ...rest
}: ButtonHTMLAttributes<HTMLButtonElement> & {
  kind?: BtnKind;
  size?: "sm" | "lg";
  icon?: ReactNode;
  loading?: boolean;
}) {
  const cls = [
    "btn",
    kind !== "default" && `btn-${kind}`,
    size === "sm" && "btn-sm",
    size === "lg" && "btn-lg",
    !children && "btn-icon",
    className,
  ]
    .filter(Boolean)
    .join(" ");
  return (
    <button type="button" className={cls} {...rest} disabled={rest.disabled || loading}>
      {loading ? <Loader2 size={14} className="animate-spin" /> : icon}
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
  footer,
  id,
}: {
  title?: ReactNode;
  icon?: ReactNode;
  right?: ReactNode;
  children: ReactNode;
  className?: string;
  bodyClass?: string;
  footer?: ReactNode;
  id?: string;
}) {
  return (
    <section className={`panel ${className}`} id={id}>
      {title !== undefined && (
        <div className="panel-head">
          {icon}
          <h2>{title}</h2>
          {right && <div className="right">{right}</div>}
        </div>
      )}
      <div className={bodyClass ?? ""}>{children}</div>
      {footer && <div className="panel-foot">{footer}</div>}
    </section>
  );
}

export function PageHead({
  title,
  sub,
  crumbs,
  actions,
  mono,
  icon,
}: {
  title: ReactNode;
  sub?: ReactNode;
  crumbs?: ReactNode;
  actions?: ReactNode;
  mono?: boolean;
  icon?: ReactNode;
}) {
  return (
    <div className="page-head">
      {icon}
      <div className="min-w-0">
        {crumbs && <nav className="crumbs">{crumbs}</nav>}
        <h1 className={mono ? "mono" : ""} style={mono ? { fontSize: 21 } : undefined}>
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
  tabs: { id: T; label: ReactNode; n?: number; icon?: ReactNode }[];
  value: T;
  onChange: (t: T) => void;
}) {
  return (
    <div className="tabs" role="tablist">
      {tabs.map((t) => (
        <button key={t.id} type="button" role="tab" aria-selected={value === t.id} onClick={() => onChange(t.id)}>
          {t.icon}
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
  sub,
  onClose,
  children,
  footer,
  wide,
}: {
  title: ReactNode;
  sub?: ReactNode;
  onClose: () => void;
  children: ReactNode;
  footer?: ReactNode;
  wide?: boolean;
}) {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const k = (e: KeyboardEvent) => e.key === "Escape" && onClose();
    window.addEventListener("keydown", k);
    const first = ref.current?.querySelector<HTMLElement>(
      ".dialog-body input, .dialog-body select, .dialog-body textarea, .dialog-body button",
    );
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
            icon={<X size={16} />}
          />
        </div>
        {sub && <div className="dialog-sub">{sub}</div>}
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
          icon={<X size={16} />}
        />
      </div>
      <div className="dialog-body flex-1">{children}</div>
    </aside>
  );
}

/** Empty is the small in-panel empty line. */
export function Empty({ title, children }: { title: string; children?: ReactNode }) {
  return (
    <div className="empty">
      <strong>{title}</strong>
      {children}
    </div>
  );
}

/** EmptyState is a page-level empty state: what this is, and what to do first. */
export function EmptyState({
  icon,
  title,
  children,
  actions,
}: {
  icon: ReactNode;
  title: string;
  children?: ReactNode;
  actions?: ReactNode;
}) {
  return (
    <div className="empty-state">
      <div className="ring">{icon}</div>
      <h3>{title}</h3>
      {children && <p>{children}</p>}
      {actions && <div className="flex flex-wrap justify-center gap-2">{actions}</div>}
    </div>
  );
}

export function Field({
  label,
  hint,
  error,
  children,
}: {
  label: string;
  hint?: ReactNode;
  error?: ReactNode;
  children: ReactNode;
}) {
  return (
    <label className="field">
      <span>{label}</span>
      {children}
      {error ? <div className="error">{error}</div> : hint && <div className="hint">{hint}</div>}
    </label>
  );
}

/** Menu is a "more actions" button with a popover list. */
export function Menu({
  label = "More actions",
  items,
  trigger,
}: {
  label?: string;
  trigger?: ReactNode;
  items: ({ label: string; icon?: ReactNode; onClick: () => void; danger?: boolean } | "sep")[];
}) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (!open) return;
    const down = (e: MouseEvent) => !ref.current?.contains(e.target as Node) && setOpen(false);
    const key = (e: KeyboardEvent) => e.key === "Escape" && setOpen(false);
    window.addEventListener("mousedown", down);
    window.addEventListener("keydown", key);
    ref.current?.querySelector<HTMLElement>(".menu button")?.focus();
    return () => {
      window.removeEventListener("mousedown", down);
      window.removeEventListener("keydown", key);
    };
  }, [open]);
  return (
    <div className="relative" ref={ref}>
      {trigger ? (
        <span onClickCapture={() => setOpen((o) => !o)}>{trigger}</span>
      ) : (
        <Button
          aria-label={label}
          aria-haspopup="menu"
          aria-expanded={open}
          icon={<MoreHorizontal size={16} />}
          onClick={() => setOpen((o) => !o)}
        />
      )}
      {open && (
        <div className="menu" role="menu">
          {items.map((it, i) =>
            it === "sep" ? (
              <hr key={`sep${i}`} />
            ) : (
              <button
                key={it.label}
                type="button"
                role="menuitem"
                className={it.danger ? "danger" : ""}
                onClick={() => {
                  setOpen(false);
                  it.onClick();
                }}
              >
                {it.icon}
                {it.label}
              </button>
            ),
          )}
        </div>
      )}
    </div>
  );
}

/** SaveBar shows only when there are unsaved changes. */
export function SaveBar({
  dirty,
  onSave,
  onDiscard,
  saving,
  label = "Save changes",
  note = "You have unsaved changes",
}: {
  dirty: boolean;
  onSave: () => void;
  onDiscard: () => void;
  saving?: boolean;
  label?: string;
  note?: string;
}) {
  if (!dirty) return null;
  return (
    <div className="savebar" role="region" aria-label="Unsaved changes">
      <span className="dot warn" />
      <span className="grow font-medium">{note}</span>
      <Button kind="ghost" onClick={onDiscard}>
        Discard
      </Button>
      <Button kind="ink" onClick={onSave} loading={saving}>
        {label}
      </Button>
    </div>
  );
}

const avatarTones = [
  ["var(--info-bg)", "var(--info)"],
  ["var(--ok-bg)", "var(--ok)"],
  ["var(--warn-bg)", "var(--warn)"],
  ["var(--teal-bg)", "var(--teal)"],
  ["var(--bad-bg)", "var(--bad)"],
  ["var(--neutral-bg)", "var(--text-2)"],
];
/** Avatar is a letter tile with a colour fixed by the name. */
export function Avatar({ name, size }: { name: string; size?: "sm" | "lg" }) {
  let h = 0;
  for (const c of name) h = (h * 31 + c.charCodeAt(0)) >>> 0;
  const [bg, fg] = avatarTones[h % avatarTones.length];
  return (
    <span className={`avatar${size ? ` ${size}` : ""}`} style={{ background: bg, color: fg }} aria-hidden>
      {name.replace(/[^a-z0-9]/gi, "").slice(0, 2) || "?"}
    </span>
  );
}

export function StatCard({
  icon,
  label,
  value,
  foot,
  onClick,
}: {
  icon: ReactNode;
  label: string;
  value: ReactNode;
  foot?: ReactNode;
  onClick?: () => void;
}) {
  const inner = (
    <>
      <div className="label">
        {icon}
        {label}
      </div>
      <div className="value">{value}</div>
      {foot && <div className="foot">{foot}</div>}
    </>
  );
  return onClick ? (
    <button type="button" className="panel card-link stat-card text-left" onClick={onClick}>
      {inner}
    </button>
  ) : (
    <div className="panel stat-card">{inner}</div>
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
    setTimeout(() => setList((l) => l.filter((t) => t.id !== id)), bad ? 6000 : 3200);
  }, []);
  return (
    <ToastCtx.Provider value={push}>
      <ConfirmProvider>{children}</ConfirmProvider>
      <div className="toast-wrap" role="status" aria-live="polite">
        {list.map((t) => (
          <div key={t.id} className={`toast${t.bad ? " bad" : ""}`}>
            {t.bad ? <CircleAlert size={15} /> : <CheckCircle2 size={15} />}
            {t.text}
          </div>
        ))}
      </div>
    </ToastCtx.Provider>
  );
}
export const useToast = () => useContext(ToastCtx);

// Confirm: a real dialog instead of the browser's confirm().
type ConfirmOpts = { title: string; body?: ReactNode; action: string; danger?: boolean };
const ConfirmCtx = createContext<(o: ConfirmOpts) => Promise<boolean>>(async () => false);
function ConfirmProvider({ children }: { children: ReactNode }) {
  const [req, setReq] = useState<(ConfirmOpts & { resolve: (v: boolean) => void }) | null>(null);
  const ask = useCallback((o: ConfirmOpts) => new Promise<boolean>((resolve) => setReq({ ...o, resolve })), []);
  const done = (v: boolean) => {
    req?.resolve(v);
    setReq(null);
  };
  return (
    <ConfirmCtx.Provider value={ask}>
      {children}
      {req && (
        <Dialog
          title={req.title}
          onClose={() => done(false)}
          footer={
            <>
              <Button onClick={() => done(false)}>Cancel</Button>
              <Button kind={req.danger ? "bad" : "ink"} onClick={() => done(true)}>
                {req.action}
              </Button>
            </>
          }
        >
          <div className="text-2">{req.body}</div>
        </Dialog>
      )}
    </ConfirmCtx.Provider>
  );
}
export const useConfirm = () => useContext(ConfirmCtx);

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
  return (
    <div className="banner bad" role="alert">
      <CircleAlert size={15} className="mt-0.5 flex-none" />
      <span>{error instanceof Error ? error.message : String(error)}</span>
    </div>
  );
}

export function Loading() {
  return (
    <div className="empty flex items-center justify-center gap-2">
      <Loader2 size={15} className="animate-spin" /> Loading…
    </div>
  );
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
        width: 34,
        height: 20,
        borderRadius: 999,
        border: 0,
        padding: 2,
        background: checked ? "var(--accent)" : "var(--line-strong)",
        cursor: "pointer",
        flex: "none",
        transition: "background .15s",
      }}
    >
      <span
        style={{
          display: "block",
          width: 16,
          height: 16,
          borderRadius: "50%",
          background: "#fff",
          boxShadow: "0 1px 2px rgba(0,0,0,.2)",
          transform: checked ? "translateX(14px)" : "none",
          transition: "transform .15s",
        }}
      />
    </button>
  );
}
