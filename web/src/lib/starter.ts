// The commented starter the editor opens for "Write it yourself".
export function starterTemplate() {
  return `// A tool has two parts.
// 1. \`tool\` is what agents see. Rename it
//    (lowercase_with_underscores) and say when to use it.
export const tool = {
  name: "new_tool",
  description: "What it returns, in one line.",
  useWhen: "When an agent should pick this tool",
  inputSchema: {
    type: "object",
    properties: { id: { type: "string" } },
    required: ["id"],
  },
};

// 2. The function calls your applications and returns
//    only what an agent needs. Type an application's name
//    and a dot (crm.) to see its requests, or click one
//    on the left to insert a call.
export default async function (input: Input) {
  return { id: input.id };
}
`;
}
