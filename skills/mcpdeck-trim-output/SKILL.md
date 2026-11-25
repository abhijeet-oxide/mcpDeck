---
name: mcpdeck-trim-output
description: Use when an mcpDeck tool returned much more than the task needed (a long list, big nested objects, long text, or over roughly 2,000 tokens), when mcpDeck adds a "large response" hint, or when the user asks to cut tokens or make a tool faster. Measures where the bytes go and reports which fields you used so a person can approve a smaller output.
---

# Trim a tool's output

Smaller outputs make every later call cheaper and faster, for every agent
using the tool. You don't change the tool yourself here: you measure, then
report what you used. A person approves the change, because another agent
may need a field you didn't.

## 1. Measure

Save the tool's response to a file and run this skill's script (Node 18+):

```sh
node scripts/field-sizes.mjs response.json
```

It lists the biggest fields by bytes, with notes such as `always empty`,
`links only`, `long text` and `up to 200 items`. Then try the fields you
actually used:

```sh
node scripts/field-sizes.mjs response.json --keep "total,issues[].key,issues[].fields.summary"
```

It prints the size after keeping only those, and the `return` statement that
would produce it. Paths use `.` for fields and `[]` for list items.

If you can't run scripts, skip this step and go by what you read.

## 2. Report what you used

Call `report_field_usage` once per tool, after you've finished using its
output:

```json
{
  "tool": "search_tickets",
  "version": 3,
  "used": ["total", "issues[].key", "issues[].fields.summary"],
  "unused": ["issues[].fields.description", "issues[].self", "issues[].fields.watches"],
  "note": "Needed ticket keys and titles to list open tickets. Description is 44% of the response."
}
```

- `used`: every field your answer depended on. Be complete: anything not
  listed may be proposed for removal.
- `unused`: fields you read past. Only list what you're sure about.
- Include `--keep` results in `note` if you measured them.

## What happens next

mcpDeck combines reports from every agent using the tool. If no agent uses a
field, it suggests removing it. If some agents use it and others don't, it
suggests a smaller output only for the agents that don't. Either way a
person approves the change before anything is different. You'll get the
smaller tool in a later session.

## If you can build tools

You still report instead of editing a shared tool's output. If you need a
much smaller version for your own task, build a separate tool (see
**mcpdeck-build-tool**) and use `--keep` to write its `return`.
