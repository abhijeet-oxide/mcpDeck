import { BookOpen, ExternalLink } from "lucide-react";
import { LEVELS } from "../components/Findings";
import { Chip, PageHead } from "../components/ui";

type Rule = { level: "must" | "avoid" | "tip"; title: string; why: string; check: string };

export const GUIDE_SECTIONS: { title: string; intro: string; rules: Rule[] }[] = [
  {
    title: "Build tools for tasks, not endpoints",
    intro:
      "An agent that has to log in, list applications, copy an ID, list images and then fetch vulnerabilities pays for every response and can copy the wrong ID. One tool that runs the chain and returns the answer is faster and cheaper.",
    rules: [
      {
        level: "must",
        title: "More than 40 tools in a toolkit",
        why: "Cursor sends at most 40 tools, and every client picks worse as the list grows.",
        check: "Split the toolkit by task, or merge tools that are steps of one job.",
      },
      {
        level: "must",
        title: "tools/list over 10,000 tokens",
        why: "Agents pay this in every conversation before doing any work.",
        check: "Shorten descriptions, drop unused tools, split the toolkit.",
      },
      {
        level: "avoid",
        title: "More than 15 tools",
        why: "Similar tools blur together and agents call the wrong one.",
        check: "Keep a toolkit to one job.",
      },
      {
        level: "avoid",
        title: "Most tools wrap one request",
        why: "The agent does the chaining, with a round trip and a full response per step.",
        check: "Chain the requests in one tool (Visual mode → add steps).",
      },
      {
        level: "avoid",
        title: "Two tools call the same requests",
        why: "Agents can't tell which to pick.",
        check: "Merge them, with an input that selects the variant.",
      },
    ],
  },
  {
    title: "Names and descriptions decide which tool gets picked",
    intro: "Agents read the name first, then the description. Both should say what comes back and when to use it.",
    rules: [
      {
        level: "must",
        title: "Generic name",
        why: "new_tool, get_data or run say nothing about the result.",
        check: "Verb + object: find_vulnerabilities, get_customer_summary.",
      },
      {
        level: "must",
        title: "Empty or starter description",
        why: "The description is how an agent decides to call the tool.",
        check: "One sentence: what it returns and what it needs.",
      },
      {
        level: "avoid",
        title: "Write tool without “don't use when”",
        why: "Write tools change data; agents should know when not to call them.",
        check: "Add dontUseWhen.",
      },
      {
        level: "tip",
        title: "No “use when” in a bigger toolkit",
        why: "A short use-when stops agents mixing tools up.",
        check: "Add useWhen: the question this tool answers.",
      },
    ],
  },
  {
    title: "Inputs agents can fill correctly",
    intro: "Every input is a guess the agent has to make. Make the guesses easy, and bind the rest.",
    rules: [
      {
        level: "avoid",
        title: "Vague parameter names (id, name, q)",
        why: "With several IDs in context the agent picks the wrong one.",
        check: "Specific names: app_name, image_id.",
      },
      {
        level: "avoid",
        title: "No required parameters",
        why: "Agents call with nothing and get an error or everything.",
        check: "Mark what the tool can't work without as required.",
      },
      {
        level: "avoid",
        title: "More than 8 parameters",
        why: "Agents fill long forms badly.",
        check: "Bind fixed values as toolkit variables.",
      },
      {
        level: "tip",
        title: "Parameters without descriptions",
        why: "One line with an example cuts wrong inputs.",
        check: "Add description and examples.",
      },
    ],
  },
  {
    title: "Return only what the task needs",
    intro:
      "The output is read into the agent's context on every call. Most API responses carry dozens of fields an agent never uses.",
    rules: [
      {
        level: "must",
        title: "Output over 25,000 tokens",
        why: "Claude Code cuts longer tool results, so the agent sees part of the answer.",
        check: "Filter, page, and keep only needed fields.",
      },
      {
        level: "must",
        title: "Credentials in the output",
        why: "Tokens and passwords would sit in the agent's context.",
        check: "Use them inside the tool, or set the application's auth.",
      },
      {
        level: "avoid",
        title: "Output over 2,000 tokens",
        why: "Large results crowd out the rest of the conversation.",
        check: "Untick fields in the Output step.",
      },
      {
        level: "avoid",
        title: "Returns the whole response",
        why: "Every field is paid for on every call.",
        check: "pick(res, [...]) or the Output step.",
      },
      {
        level: "avoid",
        title: "More than 25 fields, or lists over 50 items",
        why: "Agents rarely need more than a handful of fields or items.",
        check: "Add a limit input with a small default.",
      },
      {
        level: "avoid",
        title: "Encoded blobs",
        why: "Base64 is expensive and meaningless to an agent.",
        check: "Leave it out or return a short reference.",
      },
      {
        level: "tip",
        title: "Deep nesting, empty fields, long text, opaque IDs",
        why: "All cost tokens without helping.",
        check: "Flatten, drop empties, summarise, return names next to IDs.",
      },
    ],
  },
];

export const SOURCES = [
  {
    label: "Anthropic: Writing effective tools for agents",
    href: "https://www.anthropic.com/engineering/writing-tools-for-agents",
  },
  {
    label: "Anthropic: Code execution with MCP",
    href: "https://www.anthropic.com/engineering/code-execution-with-mcp",
  },
  { label: "Cloudflare: Code Mode", href: "https://blog.cloudflare.com/code-mode/" },
  { label: "MCP specification: tools", href: "https://modelcontextprotocol.io/specification/2025-06-18/server/tools" },
];

/** GuidelinesBody is the reference text, shown on its own page and in each toolkit (GUIDE-1). */
export function GuidelinesBody() {
  return (
    <div className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_300px]">
      <div className="flex min-w-0 flex-col gap-5">
        {GUIDE_SECTIONS.map((sec, i) => (
          <section key={sec.title} className="panel">
            <div className="px-5 pt-4 pb-3">
              <div className="muted mb-1 text-[12px] font-semibold">{i + 1}</div>
              <h2 className="text-[15px] font-semibold">{sec.title}</h2>
              <p className="text-2 m-0 mt-1 text-[13px]">{sec.intro}</p>
            </div>
            <table className="table">
              <tbody>
                {sec.rules.map((r) => {
                  const L = LEVELS[r.level];
                  return (
                    <tr key={r.title}>
                      <td style={{ width: 110 }}>
                        <Chip tone={L.tone}>{L.label}</Chip>
                      </td>
                      <td>
                        <div className="font-medium">{r.title}</div>
                        <div className="muted text-[12.5px]">{r.why}</div>
                      </td>
                      <td className="text-2 text-[12.5px]" style={{ width: "34%" }}>
                        {r.check}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </section>
        ))}
      </div>
      <aside className="flex flex-col gap-4">
        <div className="panel p-5">
          <h3 className="mb-2 text-[13.5px] font-semibold">How mcpDeck uses these</h3>
          <ul className="text-2 m-0 pl-4 text-[13px]">
            <li className="mb-1.5">Each toolkit shows a score and its findings on its Guidelines tab.</li>
            <li className="mb-1.5">
              The tool editor checks the draft as you type and shows what saving would do to the toolkit.
            </li>
            <li className="mb-1.5">
              Every run of a request or tool is analysed: tokens, fields, lists, blobs, credentials.
            </li>
            <li>No model is involved; the same input always gives the same findings.</li>
          </ul>
        </div>
        <div className="panel p-5">
          <h3 className="mb-2 text-[13.5px] font-semibold">Sources</h3>
          <ul className="m-0 flex list-none flex-col gap-2 p-0 text-[13px]">
            {SOURCES.map((s) => (
              <li key={s.href}>
                <a href={s.href} target="_blank" rel="noreferrer" className="inline-flex items-start gap-1.5">
                  {s.label} <ExternalLink size={12} className="mt-1 flex-none" />
                </a>
              </li>
            ))}
          </ul>
        </div>
      </aside>
    </div>
  );
}

export function Guidelines() {
  return (
    <div className="page">
      <PageHead
        icon={
          <span className="head-icon">
            <BookOpen size={18} />
          </span>
        }
        title="MCP guidelines"
        sub="What makes a toolkit good for agents. mcpDeck checks these while you build; a must-fix breaks agents or clients, an avoid costs tokens or wrong picks."
      />
      <GuidelinesBody />
    </div>
  );
}
