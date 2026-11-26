import { AppWindow, ArrowRight, Bot, ChartNoAxesColumn, Lock, MonitorSmartphone, ToolCase } from "lucide-react";
import { ClientIcon } from "./ClientIcon";

const parts = [
  {
    icon: AppWindow,
    tone: "",
    title: "Applications",
    what: "The systems you already have",
    body: "Import a Postman collection, an OpenAPI file or curl. Send requests and store credentials, like in Postman.",
  },
  {
    icon: ToolCase,
    tone: "teal",
    title: "Toolkits",
    what: "Tools grouped for one job",
    body: "A tool is a short TypeScript function over your applications. A toolkit is a set of tools, served as one MCP server.",
  },
  {
    icon: Bot,
    tone: "warn",
    title: "Agents",
    what: "The AI clients you connect",
    body: "Add a toolkit to VS Code, Claude Code or Cursor in one step. Each agent may read only, or read and write.",
  },
];

/** HowItWorks explains the three core ideas as a left-to-right flow. */
export function HowItWorks() {
  return (
    <div className="flow">
      {parts.map((p, i) => (
        <div key={p.title} className="contents">
          <div className="panel flex flex-col gap-2 p-4">
            <div className="flex items-center gap-2.5">
              <span className={`icon-tile ${p.tone}`}>
                <p.icon size={16} />
              </span>
              <div>
                <div className="font-semibold">{p.title}</div>
                <div className="muted text-[12px]">{p.what}</div>
              </div>
            </div>
            <p className="text-2 m-0 text-[13px] leading-relaxed">{p.body}</p>
            {p.title === "Agents" && (
              <div className="mt-auto flex items-center gap-2 pt-1">
                <ClientIcon client="vscode" size={16} />
                <ClientIcon client="claude-code" size={16} />
                <ClientIcon client="cursor" size={16} />
              </div>
            )}
          </div>
          {i < parts.length - 1 && (
            <div className="flow-arrow" aria-hidden>
              <ArrowRight size={18} />
            </div>
          )}
        </div>
      ))}
    </div>
  );
}

export function Promises() {
  const items = [
    { icon: Lock, text: "Keys are encrypted at rest and used only inside mcpDeck's proxy; they never reach an agent." },
    { icon: MonitorSmartphone, text: "One binary, no cloud account. Your APIs are called from where mcpDeck runs." },
    { icon: ChartNoAxesColumn, text: "Every call is recorded with its time and tokens." },
  ];
  return (
    <ul className="m-0 grid list-none gap-3 p-0 sm:grid-cols-3">
      {items.map(({ icon: I, text }) => (
        <li key={text} className="text-2 flex items-start gap-2.5 text-[13px]">
          <I size={16} className="muted mt-0.5 flex-none" />
          {text}
        </li>
      ))}
    </ul>
  );
}
