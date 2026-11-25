import { BookOpen, Bug, CirclePlay, ExternalLink, GitPullRequest, Package, Scale, Star } from "lucide-react";
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

export function About() {
  const info = useInfo();
  return (
    <div className="page" style={{ maxWidth: 1120 }}>
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
              mcpDeck sits between AI agents and your APIs, including old ones. Import an API the way you would in
              Postman, write small tools that return only what's needed, and give them to Claude Code, VS Code or
              Cursor. Keys stay on this computer and every call is recorded.
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
