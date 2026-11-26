import { ArrowRight, CirclePlay } from "lucide-react";
import { useState } from "react";
import { HowItWorks, Promises } from "../components/Explainer";
import { startTour } from "../components/Guide";
import { Logo } from "../components/Sidebar";
import { Button, ErrorBox } from "../components/ui";
import { put } from "../lib/api";
import { useAction, useInfo } from "../lib/hooks";
import { openDemo } from "../lib/journey";
import { navigate } from "../lib/router";

/** Welcome is the first thing a new user sees: what mcpDeck is, and an
 *  invitation to take the demo tour before setting up their own (UI-11). */
export function Welcome() {
  const info = useInfo();
  const [err, setErr] = useState<unknown>(null);
  const [opening, setOpening] = useState(false);
  const seen = useAction(() => put("/api/settings", { welcomeSeen: "true" }));
  const tour = async () => {
    setErr(null);
    if (info.data?.demo) return startTour();
    setOpening(true);
    try {
      await openDemo(true);
      seen.mutate(undefined);
    } catch (e) {
      setErr(e);
    } finally {
      setOpening(false);
    }
  };
  const own = () => seen.mutate(undefined, { onSuccess: () => navigate("/apps/new") });
  return (
    <div className="page" style={{ maxWidth: 1040 }}>
      <section className="hero mb-8 px-8 pt-12 pb-10 text-center">
        <div className="hero-grid" />
        <div className="relative">
          <div className="mb-5 flex justify-center">
            <Logo size={48} />
          </div>
          <div className="muted mb-2 text-[12.5px] font-medium">Welcome to mcpDeck</div>
          <h1 className="mx-auto mb-3 max-w-[640px] text-[30px] leading-[38px] font-semibold tracking-[-0.03em]">
            Turn the APIs you already have into tools your AI agents can use safely
          </h1>
          <p className="text-2 mx-auto mb-7 max-w-[560px] text-[14.5px] leading-relaxed">
            Import an API, write a small tool that returns only what's needed, and add it to VS Code, Claude Code or
            Cursor. Your keys stay on this computer.
          </p>
          <div className="flex flex-wrap justify-center gap-3">
            <Button kind="ink" size="lg" icon={<CirclePlay size={16} />} loading={opening} onClick={tour}>
              Take the 2-minute tour
            </Button>
            <Button size="lg" onClick={own}>
              Set up my own <ArrowRight size={15} />
            </Button>
          </div>
          <p className="muted mt-4 mb-0 text-[12.5px]">
            The tour opens a demo with sample applications in a new tab. Your own data stays empty until you add
            something.
          </p>
          <div className="mx-auto mt-3 max-w-md">
            <ErrorBox error={err || seen.error} />
          </div>
        </div>
      </section>
      <h2 className="mb-1 text-[16px] font-semibold">How it works</h2>
      <p className="muted mt-0 mb-4">Three ideas, in the order you set them up.</p>
      <HowItWorks />
      <div className="panel mt-6 px-5 py-4">
        <Promises />
      </div>
    </div>
  );
}
