---
name: issue-implement
description: >
  Browse tracked issues and pick one to implement. Lists issues
  via bluectl, reads the implementation plan from the issue notes,
  and produces an actionable plan to start working on it.
allowed-tools: Bash(bluectl:*) Read Write Edit Glob Grep Agent
---

# Issue Implement Skill

You help the user pick a tracked issue and turn it into work.

## Conversation flow

### Step 1 - List issues

Ask the user which package to list issues for.

First, list issues that have an implementation plan ready:

```
bluectl issue list <package> -f ready=true
```

Then list all remaining open issues:

```
bluectl issue list <package>
```

Present both lists to the user, clearly separating them:

**Ready to implement** (have an implementation plan):
<table from the ready=true filter>

**All open issues:**
<table from the unfiltered list>

Ask which issue they want to work on. Recommend starting with
a "ready" issue since those already have a plan.

### Step 2 - Fetch the issue

Run `bluectl issue get <id>` with the chosen issue ID.

Parse the output. The `notes` field may contain:
- A plain description (older issues)
- A structured plan with "Relevant code" and "Implementation plan"
  sections (issues created by `/issue-create`)

### Step 3 - Assess the plan

**If the issue already has an implementation plan** in its notes:
- Present the plan to the user.
- Ask if they want to proceed as-is, adjust it, or do fresh
  research first.

**If the issue has no implementation plan:**
- Summarize the issue for the user.
- Ask which repositories to look at (default: current working
  directory).
- Spawn an Explore agent to research the relevant code, looking
  for files, types, and patterns related to the issue.
- Produce an implementation plan based on the research.
- Present it to the user for confirmation.

### Step 4 - Produce the final plan

Once the user confirms, output the plan the implementing agent
should follow:

```
## Implement: <issue-id> - <subject>

### Context
<brief summary of what the issue asks for>

### Implementation steps
1. <concrete step referencing specific files/symbols>
2. ...

### Verification
- <how to verify each step works: tests to run, commands to check, etc.>

### Close the issue
Once all steps are verified, run:
`bluectl issue close <issue-id>`
```

The implementing agent (or the user) takes it from here.

## Rules

- Be concise. Ask one question at a time.
- If the user provides an issue ID directly (e.g. `/issue-implement PKG-4`),
  skip straight to Step 2.
- When an issue already contains a plan, prefer using it over
  doing redundant research. Only re-research if the user asks
  or the plan references files that no longer exist.
- Implementation steps must reference real files and symbols,
  not hypothetical ones.
- Always include a verification section so the agent knows how
  to confirm the work is correct.
