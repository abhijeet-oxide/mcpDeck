import { Plug } from "lucide-react";

/** Small marks for the MCP clients mcpDeck connects to. Lucide has no brand
 *  icons, so these are simplified inline SVGs at 1:1, sized like Lucide. */
export function ClientIcon({ client, size = 18 }: { client: string; size?: number }) {
  switch (client) {
    case "claude-code":
    case "claude-desktop":
    case "claude":
      return <ClaudeMark size={size} />;
    case "vscode":
      return <VSCodeMark size={size} />;
    case "cursor":
      return <CursorMark size={size} />;
    default:
      return <Plug size={size} strokeWidth={1.75} />;
  }
}

function ClaudeMark({ size }: { size: number }) {
  const rays = Array.from({ length: 10 }, (_, i) => i * 36);
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" aria-hidden role="img">
      <title>Claude</title>
      {rays.map((r, i) => (
        <rect
          key={r}
          x="11"
          y={i % 2 ? 3.6 : 2}
          width="2"
          height={i % 2 ? 8.4 : 10}
          rx="1"
          fill="#D97757"
          transform={`rotate(${r} 12 12)`}
        />
      ))}
    </svg>
  );
}

function VSCodeMark({ size }: { size: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" aria-hidden role="img">
      <title>VS Code</title>
      <path d="M17.2 2.2 22 4.5v15l-4.8 2.3-9.4-8.6-4.1 3.1L2 15.6V8.4l1.7-.7 4.1 3.1z" fill="#0065A9" />
      <path d="M17.2 2.2 22 4.5v15l-4.8 2.3V2.2z" fill="#1F9CF0" />
      <path d="M7.8 12 17.2 4.6v14.8z" fill="#fff" opacity=".9" />
      <path d="M17.2 6.8 11.1 12l6.1 5.2z" fill="#0078D4" />
    </svg>
  );
}

function CursorMark({ size }: { size: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" aria-hidden role="img">
      <title>Cursor</title>
      <path d="M12 1.8 21 7v10l-9 5.2L3 17V7z" fill="var(--text)" />
      <path d="M12 12 21 7l-9-5.2z" fill="var(--text-2)" opacity=".75" />
      <path d="M3.2 7.1 12 12v10.1l8.8-15.1z" fill="var(--surface)" opacity=".92" />
    </svg>
  );
}

export const clientLabels: Record<string, string> = {
  "claude-code": "Claude Code",
  "claude-desktop": "Claude Desktop",
  vscode: "VS Code",
  cursor: "Cursor",
  other: "Other client",
};
