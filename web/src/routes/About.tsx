import { BookOpen, Bug, GitPullRequest, Package, Scale, Star } from "lucide-react";
import { Logo } from "../components/Sidebar";
import { PageHead, Panel } from "../components/ui";
import { useInfo } from "../lib/hooks";

const repo = "https://github.com/abhijeet-oxide/mcpDeck";
const links = [
  { icon: Star, label: "Star on GitHub", href: repo },
  { icon: Bug, label: "Report a bug", href: `${repo}/issues/new?template=bug.yml` },
  { icon: BookOpen, label: "Docs", href: `${repo}/tree/main/docs` },
  { icon: Package, label: "Releases", href: `${repo}/releases` },
  { icon: GitPullRequest, label: "Contribute", href: `${repo}/blob/main/AGENTS.md` },
  { icon: Scale, label: "License", href: `${repo}/blob/main/LICENSE` },
];

export function About() {
  const info = useInfo();
  return (
    <div className="page" style={{ maxWidth: 760 }}>
      <PageHead title="About mcpDeck" sub={`Version ${info.data?.version ?? ""}`} />
      <Panel bodyClass="panel-body flex gap-4">
        <Logo size={44} />
        <div className="text-2 text-[13.5px]">
          <p className="mt-0">mcpDeck sits between AI agents and your APIs, including old ones.</p>
          <p>
            You import APIs the way you would in Postman, keep their keys encrypted on this computer, and write small
            TypeScript tools that call them.
          </p>
          <p>
            Each toolkit is one MCP server for Claude Code, VS Code, Cursor or Claude Desktop. Agents use the tools
            they're given, and can build new ones through a builder subagent within the limits you set.
          </p>
          <p className="mb-0">
            Every call is recorded with its time and tokens, so you can see what each tool costs and make it smaller.
          </p>
        </div>
      </Panel>
      <div className="mt-4 grid gap-2 sm:grid-cols-3">
        {links.map(({ icon: I, label, href }) => (
          <a
            key={label}
            className="panel flex items-center gap-2 p-3"
            href={href}
            target="_blank"
            rel="noreferrer"
            style={{ color: "var(--text)" }}
          >
            <I size={15} className="muted" /> {label}
          </a>
        ))}
      </div>
      <p className="muted mt-3 text-[12.5px]">
        These links open in your browser. mcpDeck itself makes no calls to GitHub.
      </p>
    </div>
  );
}
