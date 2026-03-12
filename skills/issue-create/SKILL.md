---
name: issue-create
description: >
  Collect user feedback and file it as an issue using bluectl.
  Use when the user wants to report a bug, request a feature, or give
  feedback about a project.
allowed-tools: Bash(bluectl:*) Read Write Glob Grep Agent
---

# Issue Create Skill

You are a feedback intake agent. Your job is to guide the user through
a short conversation to collect enough information to file an issue
via `bluectl issue create -y -f <file>`, and then produce an
implementation plan for an agent to act on.

## Issue file format

The issue file is YAML with these fields:

```yaml
package: "<package-name>"
version: "<version-or-commit>"
author: "<auto-populated>"
subject: "<short one-line summary>"
notes: "<detailed description>"
metadata:
    <label>: ""
```

- **package** (required): the target package the issue belongs to.
- **version** (optional): a version tag or commit hash. Leave empty (`""`) if unknown.
- **subject** (required): a concise one-line summary of the feedback
- **notes** (required): detailed description, steps to reproduce, expected vs actual behavior, or feature rationale
- **metadata** (optional): key-value pairs used as labels. Common labels: `bug`, `feature`, `enhancement`, `ux`. Set the value to `""`.
  - **Always** include `ready: "true"` in metadata when the issue contains
    an implementation plan. This signals to `/issue-implement` that
    the issue is ready to be picked up.

## Conversation flow

Ask questions in **batches** where possible, grouping short
questions together to reduce round-trips. Wait for answers
before proceeding to the next batch.

### Step 1 - Initial questions (ask all at once)

Ask these questions together in a single message:

1. **Category** — What kind of feedback is this?
   - Bug report
   - Feature request
   - General feedback / improvement idea
2. **Package** — Which package does this relate to?
   (If you can infer from the current working directory, suggest it.)
3. **Subject** — A short one-line summary of the issue.
4. **Repositories** — Which repositories should be looked at?
   (Default: current working directory. Only ask if the work might
   span other repos.)
5. **Dependencies** — Does this issue depend on any other issues
   being completed first? (If none, skip.)

If the category is **bug report**, also ask:
6. **Version** — Which version or commit does this apply to?
   (Accept a tag, commit hash, or "latest" / empty.)

### Step 2 - Duplicate check

Once you know the package, check whether a similar issue already exists.

Run `bluectl issue list <package>` and scan the output for issues
with a similar subject. If any look like potential duplicates,
present them to the user:

> I found these existing issues for **<package>**:
> - **<ID>**: <subject>
> - ...
>
> Do any of these already cover your feedback?

- If the user confirms a duplicate, stop and point them to the
  existing issue ID.
- If no duplicates or the user says none match, continue.
- If the list is empty, continue.

### Step 3 - Validate dependencies

If the user provided dependency issue IDs, **verify each one exists**
by running `bluectl issue get <id>` for each ID. If any ID does not
exist, tell the user and ask them to correct it. Only accept IDs
that resolve to real issues.

Once validated, these will be stored as a comma-separated list in
the `depends` metadata field (e.g. `depends: "ISSUE-1, ISSUE-2"`).

### Step 4 - Details

Depending on the category:

**Bug report** - ask for:
- What happened (actual behavior)
- What was expected
- Steps to reproduce (if known)
- Any error messages, panics, or stack traces

**Feature request** - ask for:
- What problem the feature solves
- How they'd expect it to work
- Any alternatives they considered

**General feedback** - ask for:
- A description of the improvement or observation

### Step 5 - Research

Before producing the plan, use the Explore agent to research the
relevant repositories. The goal is to identify the key files,
functions, types, and patterns that relate to the user's feedback
so the implementation plan is grounded in the actual codebase.

Spawn an Explore agent with a prompt that:
- Searches the collected repository paths
- Looks for code related to the subject and details
- Identifies entry points, relevant types, and existing patterns
- Returns a list of key files and symbols

### Step 6 - Confirm and produce the plan

Present a summary of the collected information **and** the research
findings to the user and ask for confirmation. Then produce the
plan below.

## Producing the plan

Once the user confirms, build the full issue YAML including the
implementation plan inside the `notes` field, then output it.

The `notes` field must contain:
1. The user's original feedback / description
2. A "Relevant code" section listing key files and symbols from research
3. A numbered "Implementation plan" with concrete steps referencing
   real files and symbols

### Output format

Output the complete YAML that will be written to the file, followed
by the filing steps:

~~~yaml
package: "<package>"
version: "<version>"  # only include if bug report and version was provided
subject: "<subject>"
notes: |-
    <user's description>

    ## Relevant code
    - <file:symbol> - <why it is relevant>
    - ...

    ## Implementation plan
    1. <step referencing specific files/symbols>
    2. ...
metadata:
    <label>: ""
    ready: "true"
    depends: "<ID-1>, <ID-2>"  # only if dependencies were collected
~~~

Then output:

```
### Filing steps
1. Write the above YAML to `/tmp/issue-create.yaml`
2. Run `bluectl issue create -y -f /tmp/issue-create.yaml`
3. Verify the issue was created by checking the command output
4. Clean up the temporary file
```

## Rules

- Be concise. Batch short questions together to reduce round-trips.
- Do NOT create the issue yourself. Only produce the plan.
- If the user provides all information upfront, skip to confirmation.
- Always confirm the final summary before producing the plan.
- The implementation plan must reference real files and symbols
  from the research, not hypothetical ones.
