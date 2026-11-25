import { Trash2 } from "lucide-react";
import type { KV } from "../lib/types";
import { Button } from "./ui";

export function KVEditor({
  rows,
  onChange,
  keyLabel = "Key",
}: {
  rows: KV[];
  onChange: (r: KV[]) => void;
  keyLabel?: string;
}) {
  const list = [...rows, { key: "", value: "" }];
  const set = (i: number, kv: KV) => {
    const next = [...rows];
    if (i === rows.length) next.push(kv);
    else next[i] = kv;
    onChange(next.filter((r) => r.key || r.value));
  };
  return (
    <table className="table">
      <thead>
        <tr>
          <th style={{ width: 28 }} />
          <th>{keyLabel}</th>
          <th>Value</th>
          <th style={{ width: 36 }} />
        </tr>
      </thead>
      <tbody>
        {list.map((r, i) => (
          <tr key={i}>
            <td>
              {i < rows.length && (
                <input
                  type="checkbox"
                  aria-label="Enabled"
                  checked={!r.disabled}
                  onChange={(e) => set(i, { ...r, disabled: !e.target.checked })}
                />
              )}
            </td>
            <td>
              <input
                className="input mono"
                aria-label={keyLabel}
                value={r.key}
                placeholder={i === rows.length ? "status" : ""}
                onChange={(e) => set(i, { ...r, key: e.target.value })}
              />
            </td>
            <td>
              <input
                className="input mono"
                aria-label="Value"
                value={r.value}
                placeholder={i === rows.length ? "overdue" : ""}
                onChange={(e) => set(i, { ...r, value: e.target.value })}
              />
            </td>
            <td>
              {i < rows.length && (
                <Button
                  size="sm"
                  kind="ghost"
                  aria-label="Remove"
                  icon={<Trash2 size={13} />}
                  onClick={() => onChange(rows.filter((_, j) => j !== i))}
                />
              )}
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}
