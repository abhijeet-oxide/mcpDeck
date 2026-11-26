import {
  BookOpen,
  Bug,
  CirclePlay,
  ExternalLink,
  GitPullRequest,
  KeyRound,
  Package,
  Scale,
  Scissors,
  ShieldCheck,
  Star,
  Workflow,
} from "lucide-react";
import { HowItWorks, Promises } from "../components/Explainer";
import { startTour } from "../components/Guide";
import { Logo, REPO } from "../components/Sidebar";
import { Button } from "../components/ui";
import { useInfo } from "../lib/hooks";
import { DemoButton } from "./Home";
import { SECURITY_LIMITS, SECURITY_PROTECTS } from "./Settings";

const links = [
  { icon: Bug, label: "Report a bug", sub: "Open a new issue", href: `${REPO}/issues/new?template=bug.yml` },
  { icon: BookOpen, label: "Docs", sub: "Vision, requirements and design", href: `${REPO}/tree/main/docs` },
  { icon: Package, label: "Releases", sub: "What changed in each version", href: `${REPO}/releases` },
  { icon: GitPullRequest, label: "Contribute", sub: "How the project is built", href: `${REPO}/blob/main/AGENTS.md` },
  { icon: Scale, label: "License", sub: "Open source", href: `${REPO}/blob/main/LICENSE` },
];

const ideas = [
  {
    icon: Workflow,
    title: "Chain requests into one tool",
    text: "Log in, list applications, pick one, list its images, fetch vulnerabilities: one tool call for the agent, not five. Build it on a canvas or in TypeScript; both edit the same code.",
  },
  {
    icon: Scissors,
    title: "Return only what's needed",
    text: "Tick the fields to keep. mcpDeck shows tokens fetched versus returned, so a 40,000-token response becomes a 600-token answer.",
  },
  {
    icon: ShieldCheck,
    title: "Guided by MCP guidelines",
    text: "Every toolkit and run is checked against what makes MCP servers work well for agents: tool count, descriptions, inputs, output size, credentials.",
  },
  {
    icon: KeyRound,
    title: "Credentials stay in mcpDeck",
    text: "Keys, tokens and certificates are used inside the proxy only. Agents get answers, never secrets, and every call is recorded.",
  },
];

export function About() {
  const info = useInfo();
  return (
    <div className="page">
      <section className="hero mb-8 p-8">
        <div className="hero-grid" />
        <div className="relative flex flex-wrap items-start gap-6">
          <Logo size={56} />
          <div className="min-w-0 flex-1">
            <div className="mb-1 flex items-center gap-2">
              <h1 className="text-[26px] font-semibold tracking-[-0.03em]">mcpDeck</h1>
              <span className="tag">{info.data?.version ?? ""}</span>
            </div>
            <p className="text-2 m-0 max-w-[620px] text-[14.5px] leading-relaxed">
              mcpDeck turns the APIs you already have, including old ones, into small, fast MCP tools. Import them the
              way you would in Postman, chain requests into tools that return only what an agent needs, and give each
              toolkit to VS Code, Claude Code or Cursor as one MCP endpoint.
            </p>
            <div className="mt-5 flex flex-wrap gap-2">
              {info.data?.demo ? (
                <Button kind="ink" icon={<CirclePlay size={14} />} onClick={startTour}>
                  Take the tour
                </Button>
              ) : (
                <DemoButton />
              )}
              <a className="btn btn-ink" href={REPO} target="_blank" rel="noreferrer">
                <Star size={14} /> Star on GitHub
              </a>
            </div>
          </div>
        </div>
      </section>

      <div className="mb-10 grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        {ideas.map(({ icon: I, title, text }) => (
          <div key={title} className="panel p-5">
            <span className="icon-tile mb-3">
              <I size={17} />
            </span>
            <h3 className="mb-1.5 text-[14px] font-semibold">{title}</h3>
            <p className="text-2 m-0 text-[13px] leading-relaxed">{text}</p>
          </div>
        ))}
      </div>

      <h2 className="mb-1 text-[16px] font-semibold">How it works</h2>
      <p className="muted mt-0 mb-4">Three ideas. Activity then shows every call and what it cost.</p>
      <HowItWorks />
      <div className="panel mt-5 px-5 py-4">
        <Promises />
      </div>

      <div className="mt-8 grid gap-5 lg:grid-cols-2">
        <section>
          <h2 className="mb-3 text-[16px] font-semibold">Security</h2>
          <div className="panel p-5">
            <div className="mb-2 text-[13px] font-semibold">What mcpDeck protects</div>
            <ul className="text-2 m-0 mb-4 pl-4 text-[13px]">
              {SECURITY_PROTECTS.map((t) => (
                <li key={t} className="mb-1.5">
                  {t}
                </li>
              ))}
            </ul>
            <div className="mb-2 text-[13px] font-semibold">What it can't</div>
            <ul className="text-2 m-0 pl-4 text-[13px]">
              {SECURITY_LIMITS.map((t) => (
                <li key={t} className="mb-1.5">
                  {t}
                </li>
              ))}
            </ul>
          </div>
        </section>
        <section>
          <h2 className="mb-3 text-[16px] font-semibold">The project</h2>
          <div className="panel">
            {links.map(({ icon: I, label, sub, href }) => (
              <a
                key={label}
                className="list-row"
                href={href}
                target="_blank"
                rel="noreferrer"
                style={{ color: "var(--text)" }}
              >
                <span className="icon-tile plain">
                  <I size={15} />
                </span>
                <span className="min-w-0 flex-1">
                  <span className="block font-medium">{label}</span>
                  <span className="muted block text-[12.5px]">{sub}</span>
                </span>
                <ExternalLink size={14} className="muted" />
              </a>
            ))}
          </div>
          <p className="muted mt-3 text-[12.5px]">
            These links open in your browser. mcpDeck itself makes no calls to GitHub.
          </p>
        </section>
      </div>
    </div>
  );
}
