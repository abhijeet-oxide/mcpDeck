import { MessageSquare } from "lucide-react";
import { useState } from "react";
import { useAgents } from "../lib/hooks";
import { Button, CopyButton, Dialog } from "./ui";

export type Prompt = { title: string; text: string; needsBuild?: boolean };

/** Who can run a prompt for a toolkit (UI-10). */
export function useRunner(toolkit?: string) {
  const agents = useAgents();
  const active = (agents.data ?? []).filter((a) => !a.revokedAt);
  const forTk = active.filter((a) => !toolkit || a.grants.some((g) => g.toolkitName === toolkit));
  const builder = forTk.find((a) => a.grants.some((g) => (!toolkit || g.toolkitName === toolkit) && g.canBuild));
  return { any: forTk[0], builder };
}

export function AskAgentButton({ prompts, toolkit }: { prompts: Prompt[]; toolkit?: string }) {
  const [open, setOpen] = useState(false);
  return (
    <>
      <Button icon={<MessageSquare size={14} />} onClick={() => setOpen(true)}>
        Ask an agent
      </Button>
      {open && <AskAgentDialog prompts={prompts} toolkit={toolkit} onClose={() => setOpen(false)} />}
    </>
  );
}

function AskAgentDialog({ prompts, toolkit, onClose }: { prompts: Prompt[]; toolkit?: string; onClose: () => void }) {
  const { any, builder } = useRunner(toolkit);
  return (
    <Dialog title="Ask an agent" onClose={onClose} wide>
      <p className="muted mt-0 mb-3">
        Paste one of these into a connected agent. Each names real tools and ends with what to check. Prompts never
        contain keys.
      </p>
      <div className="flex flex-col gap-3">
        {prompts.map((p) => {
          const who = p.needsBuild ? builder : any;
          return (
            <div key={p.title} className="panel">
              <div className="panel-head">
                <h2>{p.title}</h2>
                <div className="right">
                  <CopyButton text={p.text} what="Prompt copied" />
                </div>
              </div>
              <pre className="code" style={{ border: 0, borderRadius: 0, whiteSpace: "pre-wrap" }}>
                {p.text}
              </pre>
              <div className="muted px-3.5 py-2 text-[12px]" style={{ borderTop: "1px solid var(--line)" }}>
                {who
                  ? `Paste into ${who.name}${p.needsBuild ? `: it can build in ${toolkit ?? "its toolkit"}` : ""}.`
                  : p.needsBuild
                    ? `No agent can build in ${toolkit ?? "a toolkit"} yet: connect one with "May build" on.`
                    : "No agent is connected yet: connect one first."}
              </div>
            </div>
          );
        })}
      </div>
    </Dialog>
  );
}

export function toolRequestPrompt(r: { id: string; description: string }, toolkit: string) {
  return `In mcpDeck toolkit "${toolkit}", tool request ${r.id} asks for:
"${r.description}"

Hand this to the mcpdeck-${toolkit}-builder subagent. It should search_tools first, explore with search_apis and describe_api, then test_tool and save_tool with at least one test and fulfils: "${r.id}".
Check: the reply says the tool is live (or which approval rule held it).`;
}
