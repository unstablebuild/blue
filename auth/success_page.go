// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2018-2024 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

package auth

import (
	"html"
	"strings"
)

// SuccessPageHTML renders the OAuth2 redirect success page shown to the
// user's browser when the local callback fires. The message is plain text
// shown under the headline (it's HTML-escaped before being inlined).
//
// The page is styled to match the rune.build website: dark background,
// JetBrains Mono, brand-red accent, with a minimal "IDE chrome" header to
// echo the marketing site.
//
// Callers can pass this string as the successBrowserCopy argument to
// NewClient/NewClientWithPorts. The handler also uses it as the default
// when an empty successBrowserCopy is passed in.
func SuccessPageHTML(message string) string {
	return strings.Replace(successPageTemplate, messagePlaceholder, html.EscapeString(message), 1)
}

const messagePlaceholder = "{{message}}"

// defaultSuccessHTML is rendered when a caller passes an empty
// successBrowserCopy to NewClient/NewClientWithPorts.
var defaultSuccessHTML = SuccessPageHTML("Taking you to rune.build…")

const successPageTemplate = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<meta http-equiv="refresh" content="3; url=https://rune.build/">
<title>Signed in — Rune</title>
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@400;500;600;700&display=swap" rel="stylesheet">
<style>
  :root {
    --bg: #000000;
    --panel: #0a0a0a;
    --panel-2: #121418;
    --deepest: #000000;
    --fg: #BBC2CF;
    --fg-high: #D5DCE3;
    --fg-dim: #6B7280;
    --fg-faint: #3F4450;
    --hair: #282C34;
    --hair-soft: #1C2028;
    --gold: #ECBE7B;
    --olive: #98BE65;
    --orange: #BA0E2E;
    --cursor: #BA0E2E;
    --mono: "JetBrains Mono", ui-monospace, SFMono-Regular, Menlo, monospace;
  }
  * { box-sizing: border-box; }
  html, body {
    margin: 0;
    padding: 0;
    background: var(--bg);
    color: var(--fg);
    font-family: var(--mono);
    -webkit-font-smoothing: antialiased;
    text-rendering: geometricPrecision;
    font-feature-settings: 'liga' 1, 'calt' 1;
    min-height: 100vh;
  }
  .shell { min-height: 100vh; display: flex; flex-direction: column; }
  .chrome {
    background: var(--deepest);
    border-bottom: 1px solid var(--hair);
  }
  .tabbar {
    display: flex;
    align-items: center;
    min-height: 46px;
    padding-left: 14px;
  }
  .traffic {
    display: flex; align-items: center; gap: 7px;
    padding-right: 14px;
    border-right: 1px solid var(--hair);
  }
  .traffic span {
    width: 11px; height: 11px; border-radius: 50%;
    display: inline-block;
  }
  .traffic .r { background: #BA0E2E; }
  .traffic .y { background: #ECBE7B; }
  .traffic .g { background: #98BE65; }
  .tab {
    display: inline-flex; align-items: center; gap: 8px;
    padding: 0 16px;
    color: var(--fg-high);
    font-size: 14.5px;
    letter-spacing: -0.005em;
    border-right: 1px solid var(--hair-soft);
    height: 46px;
    background: var(--bg);
    position: relative;
    user-select: none;
    text-decoration: none;
    transition: color .15s, background .15s;
  }
  .tab:hover { color: var(--gold); background: var(--panel-2); }
  .tab::before {
    content: "";
    position: absolute; left: 0; right: 0; top: 0; height: 2px;
    background: var(--orange);
  }
  .tab .dot { width: 6px; height: 6px; border-radius: 50%; background: var(--gold); }
  main {
    flex: 1;
    width: 100%;
    max-width: 1180px;
    margin: 0 auto;
    padding: 0 44px;
  }
  @media (max-width: 960px) {
    main { padding: 0 22px; }
  }
  .page {
    padding: 110px 0 140px;
    max-width: 56ch;
  }
  .label {
    display: block;
    font-size: 14.5px;
    color: var(--fg-dim);
    margin-bottom: 22px;
  }
  .label::before { content: "// "; color: var(--fg-dim); }
  .hero {
    font-size: clamp(2.6rem, 8vw, 5rem);
    font-weight: 700;
    letter-spacing: -0.04em;
    line-height: 1;
    color: var(--orange);
    margin: 0 0 18px;
  }
  .cursor {
    display: inline-block;
    width: .65ch;
    height: 0.95em;
    background: var(--cursor);
    margin-left: 6px;
    vertical-align: -0.08em;
    animation: rune-blink 1.05s steps(1) infinite;
  }
  @keyframes rune-blink { 50% { background: transparent; } }
  .lead {
    color: var(--fg);
    font-size: 1rem;
    line-height: 1.6;
    margin: 0 0 30px;
  }
  .continue {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    border: 1px solid var(--hair);
    background: var(--panel);
    color: var(--fg);
    padding: 10px 18px;
    font-size: 13.5px;
    font-family: var(--mono);
    text-decoration: none;
    border-radius: 2px;
    transition: all .15s;
  }
  .continue:hover {
    border-color: var(--fg-dim);
    color: var(--fg-high);
    background: var(--panel-2);
  }
  .footer {
    border-top: 1px solid var(--hair);
    color: var(--fg-faint);
    font-size: 12px;
    padding: 14px 44px;
    display: flex;
    justify-content: space-between;
    gap: 12px;
  }
  .footer .ok { color: var(--olive); }
  @media (max-width: 960px) {
    .footer { padding: 14px 22px; }
  }
</style>
</head>
<body>
<div class="shell">
  <header class="chrome">
    <div class="tabbar" role="presentation">
      <div class="traffic" aria-hidden="true">
        <span class="r"></span><span class="y"></span><span class="g"></span>
      </div>
      <a class="tab" aria-current="true" href="https://rune.build/">
        <span class="dot"></span><span>rune.build</span>
      </a>
    </div>
  </header>
  <main>
    <section class="page">
      <span class="label">auth</span>
      <h1 class="hero">You're in<span class="cursor" aria-hidden="true"></span></h1>
      <p class="lead">{{message}}</p>
      <a class="continue" href="https://rune.build/">Continue to rune.build →</a>
    </section>
  </main>
  <footer class="footer">
    <span><span class="ok">●</span> session established</span>
    <span>rune.build</span>
  </footer>
</div>
</body>
</html>
`
