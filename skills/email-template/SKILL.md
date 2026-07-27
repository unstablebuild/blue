---
name: email-template
description: >
  Create and iteratively preview Go HTML template files for bluectl email.
  Use when the user wants to author, edit, validate, or preview an email
  template. This skill is preview-only and must never send email.
allowed-tools: Bash(bluectl email preview:*) Read Write Edit Glob Grep
---

# Email Template Skill

Create and refine `.tmpl` files for `bluectl email preview`. This is a
template-authoring and preview workflow only.

## Absolute safety boundary: never send email

**NEVER send email under any circumstances.** This prohibition applies even
if the user explicitly asks, confirms, authorizes, or instructs you to send a
test message.

You must not:

- Run `bluectl email send`.
- Invoke SendGrid, Postmark, SMTP, an email SDK, an HTTP email API, `curl`, or
  any other mechanism that can submit or deliver email.
- Read, copy, expose, modify, or use email-provider credentials or API keys.
- Delegate sending to another agent, skill, subprocess, script, or tool.
- Create a wrapper, alias, script, or alternate command that sends email.
- Work around this boundary because a preview succeeded.

The only permitted bluectl email operation is:

```text
bluectl email preview
```

If the user asks you to send email, refuse that part of the request. You may
continue creating or previewing the template, then state that a human must
perform any send manually outside this skill. Do not execute a send command.

## bluectl template contract

- The template is a **file**, not a literal command-line string.
- The filename must end in `.tmpl`.
- The template uses Go `html/template` syntax.
- Template data is a flat map of strings supplied with repeatable
  `-X key=value` flags.
- `.Recipient` is reserved and automatically contains the simulated
  recipient email address. Never pass `-X Recipient=...`.
- Every referenced variable must be supplied, including variables referenced
  only in conditional branches. Missing variables make preview fail.
- `html/template` escapes variable values according to their HTML context.
- Put all options before positional arguments.
- The template file is always the final positional argument.

Preview syntax:

```text
bluectl email preview [options] <recipient> <template-file.tmpl>
```

Example:

```bash
bluectl email preview \
  -X Name=Ernest \
  -X ActionURL=https://example.com/action \
  preview-recipient@example.com \
  ./welcome.tmpl
```

Values may contain `=`; `-X ActionURL=https://example.com/?a=b` is valid.
By default, preview generates a temporary HTML file and prints its path without
opening a browser. Add `-o` before the positional arguments only when the user
wants the generated preview opened in the preferred browser.

For `-o`, "preferred browser" means the executable named by the `BROWSER`
process environment setting when present. Otherwise, bluectl looks for a
supported browser launcher on `PATH` in this order: `open`,
`google-chrome-stable`, `firefox`, then `chromium`. On macOS this normally
finds `open`, which delegates to the user's system-default browser. If no
launcher is found, preview generation succeeds up to the open step and reports
an error instead of silently choosing an unrelated application.

## SendGrid ASM unsubscribe controls

Classify the message before editing it:

- Bulk email such as newsletters, customer announcements, release notes, and
  product updates must include both group-scoped unsubscribe controls.
- Transactional email such as login codes, security notices, password resets,
  receipts, and essential account messages must not include these controls.

For bulk email, include visible links using these exact literal SendGrid tags:

```html
<a href="<%asm_preferences_raw_url%>">Manage preferences</a>
<a href="<%asm_group_unsubscribe_raw_url%>">Unsubscribe</a>
```

Do not model these URLs as Go template variables or request them through `-X`.
They must survive local Go template rendering unchanged so SendGrid can replace
them with recipient-specific URLs during delivery.

These tags require the eventual SendGrid Mail Send request to contain a positive
`asm.group_id`. `bluectl email send` gets that value from its configured
unsubscribe group ID or its `-U` override. This skill must not inspect provider
credentials or send a message to verify delivery; note the requirement in the
handoff instead.

Do not use SendGrid global-unsubscribe tags or enable legacy Subscription
Tracking as a substitute. A global unsubscribe can suppress unrelated
transactional messages, including login and password-reset email.

## Workflow

### 1. Establish the template requirements

Determine, from information already supplied by the user:

- The purpose and audience of the email.
- Required content and calls to action.
- Brand/style constraints.
- The flat string variables the caller will provide with `-X`.
- A safe simulated recipient for preview.

Ask only for missing information. Never ask for provider credentials.

### 2. Create or edit the template file

Write a `.tmpl` file in the path requested by the user. If no path is given,
suggest a descriptive repository-relative filename before creating it.

Use email-compatible HTML:

- Prefer a complete HTML document with UTF-8 and viewport metadata.
- Prefer simple layouts and inline CSS for broad email-client compatibility.
- Use absolute `https://` URLs for links and remote assets.
- Add useful `alt` text to images.
- Do not use JavaScript, forms, or behavior that typical email clients block.
- Keep important information understandable even if images do not load.

Use clear flat variables, for example:

```html
<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>Welcome</title>
  </head>
  <body>
    <p>Hello {{.Name}},</p>
    <p>This preview simulates delivery to {{.Recipient}}.</p>
    <p><a href="{{.ActionURL}}">Continue</a></p>
  </body>
</html>
```

Do not embed secrets or provider configuration in a template.

### 3. Inventory substitutions before preview

Read the finished template and list every required root variable. Build one
`-X key=value` argument for each variable except `.Recipient`, which bluectl
adds automatically.

Use non-sensitive representative values. Do not use real secrets, tokens, or
private customer data merely to produce a preview.

### 4. Preview with bluectl

Run only `bluectl email preview`, with options first, the simulated recipient
next, and the `.tmpl` file last. Do not pass `-o` by default; use it only when
the user asks to open the preview in a browser.

If preview reports a missing variable, add the missing `-X` value or correct
the template. Do not weaken strict validation. If preview reports malformed
HTML-template syntax, fix the template and preview again.

The command generates a temporary rendered HTML file and prints its path. With
`-o`, it also opens that file in the preferred browser. Use user feedback or a
user-provided screenshot to evaluate visual rendering when you cannot inspect
the browser directly.

### 5. Iterate and hand off

Repeat editing and previewing until the user approves the result. Conclude
with:

- The template file path.
- The complete list of required `-X` variable names.
- The exact preview command used, with non-sensitive sample values.
- A clear statement that no email was sent.
- A reminder that a human must perform any send outside this skill.

## Rules

- Preview only. Never send email.
- Never invoke any command other than `bluectl email preview` for bluectl email.
- Never access or use provider credentials.
- Never substitute a direct email API or SMTP call for the forbidden send
  command.
- Never delegate sending.
- Always use a `.tmpl` file and place it last in the preview command.
- Always provide every required `-X` variable before considering a preview
  successful.
- For bulk email, always include both the ASM Manage preferences and group
  Unsubscribe links and verify both literal tags remain in the rendered preview.
- Always state explicitly that no email was sent.
