export const tool = {
  name: "list_receipts",
  description: "Lists the receipt files in your receipts folder, newest first, with size and date.",
  useWhen: "To find the receipt for a claim; match it to list_claims by month and purpose.",
  inputSchema: { type: "object", properties: {} },
};

export default async function (input: Input) {
  const files_ = await files.list("expenses");
  return { receipts: files_.map((f) => ({ name: f.name, size: f.size, modified: f.modified })) };
}
