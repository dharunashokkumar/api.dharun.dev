package api

// The layout every HTML page shares: head, header, footer, styles and the small script
// for code tabs, copy buttons and JSON colouring. Cool near-white paper, hairlines, code
// on graphite, one cobalt signal. The fonts are served from this binary (webfonts.go);
// nothing is loaded from elsewhere: no third-party fonts, no analytics, no cookies.

// cssStamp records the design decisions (the Hallmark design skill reads it back).
const cssStamp = `
/* Hallmark · shape: Index-First (swapped from Marquee Hero: an API has to read in seconds) · genre: modern-minimal · theme: Cobalt (deal 1/3, seed free-public-rest-mcp-ef5a·b523d20f·2026-10-25·r0) · redesign: full · draw: c63a29
 * hero: Counter (/), Banner (/docs, /docs/*, /ai, /guides, /guides/*, prose) · nav: floating bar, cut corners · footer: colophon block beside three short link lists · imagery: the product (real answers rendered by this server) · signature: a real pincode request and its real answer on graphite, in the hero
 * axes: light/grotesk-sans/cool · posture: restrained · contrast: pass · fold: pass · job: sign up (browse the APIs, make the first call) · v2.0.0 */
/* Hallmark · pre-emit critique: P4 H4 E4 S4 R5 V4 · weakest: S, the headline could head any data API; the first screen carries pincode 637001 answered by this server, which only this product can show */
/* Hallmark · waive 78 · the reference pages are a Long Document of endpoints whose heads are per endpoint · the home keeps section heads at full size */
/* Hallmark · waive 77 · the facts strip under the hero and the AI band are bands of one row and an action · the API index below carries the mass */
/* Hallmark · waive 49b · a real phrase that cannot shorten, two lines at most · the guide titles linked from each API page are the guides' own names */
/* Hallmark · waive 44a · measured on the 1440x900 shot, 130px above the console and 155px below it to the facts rule · the hero is centred by its grid, the linter reads the sticky bar's offset */
/* Hallmark · waive 44c · Banner posture on the reference routes, whose first section starts by 570px · the API pages, guides and trust pages open straight into their content */
/* Hallmark · waive 79 · Banner posture on the reference routes, the space under the page head is the first section's own top · the API pages, guides and trust pages are read, not landed on */
/* Hallmark · waive 72 · the measured box is the scroll content of a pre padded 16 to 24px on every side · JSON answers on the home and API pages scroll inside their own panel */
/* Hallmark · dismissed 73 · the flagged text is the scrolled-off part of a JSON answer inside its own scroll box (pre.sample, overflow: auto), clipped and never painted under the next section */
/* Hallmark · answered F20 · the API index lists each category's real APIs two across; an odd count leaves one on its row */
`

const cssTokens = `:root {
  color-scheme: light;
  --space-3xs: 2px; --space-2xs: 4px; --space-xs: 8px; --space-sm: 12px; --space-md: 16px; --space-lg: 24px; --space-xl: 40px; --space-2xl: 64px; --space-3xl: 96px; --space-4xl: 144px;
  --text-xs: 0.8125rem; --text-sm: 0.875rem; --text-md: 1rem; --text-lg: 1.25rem; --text-xl: 1.5625rem; --text-2xl: 1.953rem; --text-3xl: 2.441rem;
  --text-display-s: clamp(2.25rem, 4vw + 0.5rem, 3.75rem); --text-display: clamp(2.75rem, 5vw + 1rem, 5.25rem);
  --text-section: clamp(2rem, 2.5vw + 1rem, 3.5rem); --text-title: clamp(1.25rem, 0.75vw + 0.875rem, 1.625rem); --text-lead: clamp(1.125rem, 0.35vw + 1rem, 1.375rem);
  --ease-out: cubic-bezier(0.16, 1, 0.3, 1); --ease-in: cubic-bezier(0.7, 0, 0.84, 0); --ease-in-out: cubic-bezier(0.65, 0, 0.35, 1);
  --dur-micro: 120ms; --dur-short: 220ms; --dur-long: 420ms;
  --color-error: oklch(52% 0.19 27); --color-success: oklch(47% 0.12 150);
  --container: 1440px; --gutter: 64px; --gap-columns: 32px; --section-gap-s: 120px; --section-gap: 144px; --section-gap-l: 176px;
  --z-base: 1; --z-raised: 10; --z-dropdown: 100; --z-sticky: 200; --z-sticky-nav: 300; --z-modal: 400; --z-toast: 500; --z-tooltip: 600;
  --color-paper: oklch(98.6% 0.005 250); --color-paper-2: oklch(96.2% 0.008 252); --color-paper-3: oklch(93% 0.011 252);
  --color-ink: oklch(21% 0.028 262); --color-ink-2: oklch(36% 0.022 260); --color-muted: oklch(48% 0.018 258);
  --color-rule: oklch(90% 0.009 252); --color-rule-2: oklch(83% 0.012 252);
  --color-graphite: oklch(21% 0.024 262); --color-graphite-2: oklch(26.5% 0.026 262); --color-graphite-ink: oklch(93% 0.010 250); --color-graphite-muted: oklch(72% 0.022 256);
  --color-accent: oklch(52% 0.19 258); --color-accent-2: oklch(76% 0.13 254); --color-accent-wash: oklch(95% 0.028 256); --color-accent-ink: oklch(98.6% 0.005 250); --color-focus: oklch(52% 0.19 258);
  --color-accent-deep: oklch(43% 0.2 262); --color-accent-edge: oklch(39% 0.16 260);
  --color-graphite-glyph-1: oklch(33% 0.03 262); --color-graphite-glyph-2: oklch(41% 0.05 260);
  --color-key-face: oklch(99.4% 0.003 250); --color-key-edge: oklch(80% 0.016 254);
  --font-display: "Familjen Grotesk", system-ui, sans-serif; --font-body: "IBM Plex Sans", system-ui, sans-serif;
  --font-outlier: "JetBrains Mono", ui-monospace, "SF Mono", Menlo, Consolas, monospace;
  --radius-card: 8px; --radius-input: 6px;
  --cut: 8px; --cut-l: 16px; --nav-h: 60px; --side-w: 232px;
  --shadow-card: 0 1px 2px oklch(21% 0.028 262 / 0.06);
  --shadow-key: 0 3px 0 0 var(--color-key-edge), 0 6px 8px -4px oklch(21% 0.028 262 / 0.28), inset 0 1px 0 oklch(100% 0 0 / 0.9);
  --shadow-key-accent: 0 3px 0 0 var(--color-accent-edge), 0 6px 8px -4px oklch(30% 0.12 260 / 0.45), inset 0 1px 0 oklch(100% 0 0 / 0.22);
  --shadow-key-press: inset 0 1px 2px oklch(21% 0.028 262 / 0.14);
  --tracking-display: -0.028em; --display-weight: 600;
}
`

const cssBase = `*, *::before, *::after { box-sizing: border-box; }
html, body { overflow-x: clip; }
html { -webkit-text-size-adjust: 100%; scrollbar-color: var(--color-muted) var(--color-paper); scroll-padding-top: calc(var(--nav-h) + var(--space-xl)); }
body { margin: 0; background: var(--color-paper); color: var(--color-ink); font-family: var(--font-body); font-size: var(--text-md); line-height: 1.55; -webkit-font-smoothing: antialiased; caret-color: var(--color-accent); }
::selection { background: color-mix(in oklab, var(--color-accent) 22%, transparent); color: var(--color-ink); }
:focus-visible { outline: 2px solid var(--color-focus); outline-offset: 2px; }
a { color: var(--color-ink); text-decoration: underline; text-decoration-color: var(--color-rule-2); text-decoration-thickness: 1px; text-underline-offset: 0.2em; transition: color var(--dur-short) var(--ease-out), text-decoration-color var(--dur-short) var(--ease-out); }
a:hover { color: var(--color-accent-deep); text-decoration-color: var(--color-accent); }
a:active { color: var(--color-accent); }
h1, h2, h3 { margin: 0; font-family: var(--font-display); font-weight: var(--display-weight); font-synthesis: none; letter-spacing: var(--tracking-display); line-height: 1.05; overflow-wrap: break-word; min-width: 0; text-wrap: balance; color: var(--color-ink); }
h1 { max-width: 20ch; font-size: var(--text-display); } h2 { max-width: 30ch; font-size: var(--text-section); } h3 { max-width: 40ch; font-size: var(--text-title); font-weight: 500; line-height: 1.2; }
.lead { font-size: var(--text-lead); line-height: 1.45; max-width: 52ch; color: var(--color-ink-2); }
p { margin: 0; text-wrap: pretty; }
p:not([class]) { max-width: 66ch; }
b, strong, th { font-weight: 500; }
img, video { max-width: 100%; height: auto; display: block; }
svg { display: block; }
[hidden] { display: none !important; }
button, input, select, textarea { font: inherit; }
.container { width: 100%; max-width: var(--container); margin-inline: auto; padding-inline: var(--gutter); }
.visually-hidden { position: absolute; width: 1px; height: 1px; overflow: hidden; clip-path: inset(50%); white-space: nowrap; }
@media (max-width: 60rem) { :root { --gutter: 24px; --section-gap-s: 80px; --section-gap: 96px; --section-gap-l: 120px; } }
@media (max-width: 40rem) { :root { --gutter: 16px; --section-gap-s: 56px; --section-gap: 72px; --section-gap-l: 96px; --gap-columns: 16px; } }
@media (prefers-reduced-motion: reduce) { *, *::before, *::after { animation-duration: 0.01ms !important; animation-iteration-count: 1 !important; transition-duration: 0.01ms !important; scroll-behavior: auto !important; } }
`

const cssPage = `
.skip { position: absolute; left: var(--space-md); top: calc(-1 * var(--space-3xl)); z-index: var(--z-toast); padding: var(--space-xs) var(--space-md); background: var(--color-ink); color: var(--color-paper); }
.skip:focus-visible { top: var(--space-md); }

/* the floating bar */
.top { position: sticky; top: var(--space-sm); z-index: var(--z-sticky-nav); margin-top: var(--space-sm); }
.bar { display: flex; align-items: center; gap: var(--space-lg); min-height: var(--nav-h); padding-inline: var(--space-md) var(--space-xs); background: var(--color-paper); border: 1px solid var(--color-rule); border-radius: var(--radius-card); box-shadow: var(--shadow-card); }
.brand { display: inline-flex; align-items: center; gap: var(--space-xs); color: var(--color-ink); font: 600 var(--text-lg)/1 var(--font-display); letter-spacing: var(--tracking-display); text-decoration: none; white-space: nowrap; }
.brand:hover { color: var(--color-ink); }
.brand svg { width: 24px; height: 24px; flex-shrink: 0; }
.brand rect { fill: var(--color-ink); } .brand path { stroke: var(--color-paper); }
.bar nav { display: flex; gap: var(--space-2xs); margin-left: auto; }
.bar nav a { display: inline-flex; align-items: center; min-height: 44px; padding-inline: var(--space-sm); border-radius: var(--radius-input); color: var(--color-ink-2); font: 500 var(--text-sm)/1 var(--font-display); text-decoration: none; white-space: nowrap; transition: background-color var(--dur-short) var(--ease-out), color var(--dur-short) var(--ease-out); }
.bar nav a:hover { background: var(--color-paper-2); color: var(--color-ink); }
.bar nav a:active { background: var(--color-paper-3); }
.bar nav a[aria-current] { color: var(--color-accent-deep); background: var(--color-accent-wash); }

/* keys */
.key { display: inline-flex; align-items: center; justify-content: center; min-height: 48px; padding: 0 var(--space-lg); border: 1px solid var(--color-rule-2); border-radius: var(--radius-input); background: var(--color-key-face); color: var(--color-ink); font: 500 var(--text-md)/1 var(--font-display); white-space: nowrap; text-decoration: none; cursor: pointer; box-shadow: var(--shadow-key); transition: transform var(--dur-micro) var(--ease-out), box-shadow var(--dur-micro) var(--ease-out); }
.key:hover { color: var(--color-ink); transform: translateY(-1px); }
.key:active { transform: translateY(3px); box-shadow: var(--shadow-key-press); }
.key:focus-visible { outline: 2px solid var(--color-focus); outline-offset: 4px; }
.key--primary { background: var(--color-accent); border-color: var(--color-accent-edge); color: var(--color-accent-ink); box-shadow: var(--shadow-key-accent); }
.key--primary:hover { color: var(--color-accent-ink); }
.key:disabled, .copy:disabled, .tabs button:disabled { opacity: 0.55; cursor: not-allowed; transform: none; }
.actions { display: flex; flex-wrap: wrap; gap: var(--space-md); margin-top: var(--space-xl); }

/* code: graphite */
code, pre { font-family: var(--font-outlier); }
code { font-size: 0.875em; background: var(--color-paper-3); color: var(--color-ink); border-radius: 4px; padding: 0.1em 0.35em; overflow-wrap: anywhere; }
pre { margin: 0; background: var(--color-graphite); color: var(--color-graphite-ink); border-radius: var(--radius-card); padding: var(--space-md) var(--space-lg); overflow-x: auto; font-size: var(--text-sm); line-height: 1.65; tab-size: 2; scrollbar-color: var(--color-graphite-muted) var(--color-graphite); }
pre code { background: none; padding: 0; color: inherit; font-size: inherit; overflow-wrap: normal; border-radius: 0; }
pre.sample { max-height: calc(16lh + var(--space-md)); overflow: auto; }
pre :focus-visible, pre:focus-visible { outline-color: var(--color-accent-2); }
.j-s, .j-n { color: var(--color-accent-2); }
.codebox { border-radius: var(--radius-card); overflow: hidden; background: var(--color-graphite); }
.codebox pre { border-radius: 0; }
.strip { display: flex; align-items: center; justify-content: space-between; gap: var(--space-sm); min-height: 52px; padding: var(--space-2xs) var(--space-2xs) var(--space-2xs) var(--space-lg); background: var(--color-graphite-2); color: var(--color-graphite-muted); font: 500 var(--text-sm)/1.2 var(--font-display); }
.copy { min-height: 44px; padding: 0 var(--space-md); border: 1px solid var(--color-graphite-glyph-2); border-radius: var(--radius-input); background: var(--color-graphite); color: var(--color-graphite-ink); font: 500 var(--text-sm)/1 var(--font-display); cursor: pointer; transition: background-color var(--dur-micro) var(--ease-out), transform var(--dur-micro) var(--ease-out); }
.copy:hover { background: var(--color-graphite-glyph-1); }
.copy:active { transform: translateY(1px); }
.copy:focus-visible { outline: 2px solid var(--color-accent-2); outline-offset: 2px; }
@supports (corner-shape: bevel) { .codebox, .console, .bar { border-radius: var(--cut-l) 4px var(--cut-l) 4px; corner-shape: bevel round bevel round; } }

/* home: the hero is a real call */
.hero { display: grid; grid-template-columns: minmax(0, 4fr) minmax(0, 7fr); gap: var(--space-2xl); align-items: center; min-height: calc(100svh - var(--nav-h) - var(--space-sm)); padding-block: var(--space-xl) var(--space-2xl); }
.hero h1 { font-size: var(--text-display); line-height: 1.02; max-width: 9ch; }
.hero .lead { margin-top: var(--space-lg); max-width: 36ch; }
.console { margin: 0; border-radius: var(--radius-card); overflow: hidden; background: var(--color-graphite); box-shadow: var(--shadow-card); }
.console .codebox { border-radius: 0; }
.console .codebox + .codebox { border-top: 1px solid var(--color-graphite-glyph-1); }
.console pre.sample { max-height: calc(15lh + var(--space-md)); }

.facts { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); padding-block: var(--space-2xl) var(--space-xl); border-top: 1px solid var(--color-rule-2); }
.facts > div { padding-inline: var(--space-lg); border-left: 1px solid var(--color-rule); }
.facts > div:first-child { padding-left: 0; border-left: 0; }
.facts > div:last-child { padding-right: 0; }
.t { font-family: var(--font-display); font-size: var(--text-title); font-weight: 500; letter-spacing: -0.01em; line-height: 1.2; }
.facts .t { margin-bottom: var(--space-xs); }
.facts p { color: var(--color-ink-2); max-width: 46ch; }

.sec { padding-block: var(--section-gap-s); }
.sec-l { padding-block: var(--section-gap) var(--section-gap-l); }
.head { display: grid; gap: var(--space-md); margin-bottom: var(--space-2xl); }
.head-split { grid-template-columns: repeat(12, minmax(0, 1fr)); column-gap: var(--gap-columns); align-items: end; }
.head-split h2 { grid-column: 1 / 6; } .head-split .lead { grid-column: 7 / 13; }

.cat { display: grid; grid-template-columns: repeat(12, minmax(0, 1fr)); column-gap: var(--gap-columns); padding-block: var(--space-lg) var(--space-xl); border-top: 1px solid var(--color-rule-2); }
.cat h3 { grid-column: 1 / 4; position: sticky; top: calc(var(--nav-h) + var(--space-xl)); align-self: start; padding-top: var(--space-sm); }
.cat ul { grid-column: 4 / 13; display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); column-gap: var(--gap-columns); margin: 0; padding: 0; list-style: none; }
.cat li a { display: grid; align-content: start; gap: var(--space-3xs); min-height: 76px; padding-block: var(--space-sm); border-top: 1px solid var(--color-rule); color: var(--color-ink); text-decoration: none; }
.cat li:nth-child(-n+2) a { border-top-color: transparent; }
.cat li strong { font: 500 var(--text-lg)/1.25 var(--font-display); letter-spacing: -0.01em; transition: color var(--dur-short) var(--ease-out); }
.cat li span { color: var(--color-muted); font-size: var(--text-sm); max-width: 48ch; }
.cat li a:hover strong { color: var(--color-accent); }

.band { padding-block: var(--section-gap-s); background: var(--color-paper-2); border-block: 1px solid var(--color-rule); }
.split { display: grid; grid-template-columns: repeat(12, minmax(0, 1fr)); column-gap: var(--gap-columns); row-gap: var(--space-xl); align-items: center; }
.split-text { grid-column: 1 / 6; display: grid; gap: var(--space-lg); justify-items: start; }
.split-visual { grid-column: 7 / 13; display: grid; gap: var(--space-md); }

.guides { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); column-gap: var(--gap-columns); margin: 0; padding: 0; list-style: none; }
.guides a { display: grid; align-content: start; gap: var(--space-xs); min-height: 100%; padding-block: var(--space-lg) var(--space-xl); border-top: 1px solid var(--color-rule-2); color: var(--color-ink); text-decoration: none; }
.guides a:hover .t { color: var(--color-accent); }
.guides .t { transition: color var(--dur-short) var(--ease-out); }
.guides span { color: var(--color-muted); max-width: 46ch; }

.faqs { display: grid; grid-template-columns: repeat(12, minmax(0, 1fr)); column-gap: var(--gap-columns); row-gap: var(--space-xl); }
.faqs h2 { grid-column: 1 / 5; position: sticky; top: calc(var(--nav-h) + var(--space-xl)); align-self: start; }
.faq-list { grid-column: 6 / 13; border-bottom: 1px solid var(--color-rule); }
.faq { border-top: 1px solid var(--color-rule); }
.faq summary { display: flex; justify-content: space-between; align-items: center; gap: var(--space-lg); min-height: 68px; padding-block: var(--space-md); cursor: pointer; list-style: none; font: 500 var(--text-lg)/1.3 var(--font-display); letter-spacing: -0.01em; }
.faq summary::-webkit-details-marker { display: none; }
.faq summary::after { content: "+"; flex-shrink: 0; color: var(--color-muted); font-weight: 500; }
.faq[open] summary::after { content: "\2212"; }
.faq summary:hover { color: var(--color-accent-deep); }
.faq p { color: var(--color-ink-2); padding-bottom: var(--space-lg); }

/* reference pages */
.page { padding-block: var(--space-2xl) var(--section-gap-s); }
.pagehead { display: grid; gap: var(--space-md); padding-block: var(--space-xl); }
.pagehead h1, .article h1 { font-size: var(--text-display-s); line-height: 1.04; max-width: 24ch; }
.pagehead .lead { max-width: 62ch; }
.chips { display: flex; flex-wrap: wrap; gap: var(--space-xs); margin-top: var(--space-md); }
.chips a { display: inline-flex; align-items: center; min-height: 44px; padding-inline: var(--space-md); border: 1px solid var(--color-rule-2); border-radius: var(--radius-input); background: var(--color-key-face); color: var(--color-ink); font: 500 var(--text-sm)/1.2 var(--font-display); text-decoration: none; transition: background-color var(--dur-short) var(--ease-out); }
.chips a:hover { background: var(--color-paper-2); color: var(--color-ink); }
.chips a:active { background: var(--color-paper-3); }

.dircat { display: grid; grid-template-columns: repeat(12, minmax(0, 1fr)); column-gap: var(--gap-columns); padding-block: var(--space-xl) var(--space-2xl); border-top: 1px solid var(--color-rule-2); }
.dircat > h2 { grid-column: 1 / 4; font-size: var(--text-2xl); position: sticky; top: calc(var(--nav-h) + var(--space-xl)); align-self: start; }
.dirs { grid-column: 4 / 13; display: grid; gap: var(--space-xl); }
.dir h3 a { color: var(--color-ink); text-decoration: none; }
.dir h3 a:hover { color: var(--color-accent); }
.dir > p { color: var(--color-muted); margin: var(--space-2xs) 0 var(--space-sm); }
.dir ul { margin: 0; padding: 0; list-style: none; font-size: var(--text-sm); }
.dir li { display: grid; grid-template-columns: minmax(0, 5fr) minmax(0, 7fr); gap: var(--space-md); padding-block: var(--space-xs); border-top: 1px solid var(--color-rule); }
.dir li code { background: none; padding: 0; color: var(--color-ink); }
.dir li span { color: var(--color-ink-2); }

.docs { display: grid; grid-template-columns: var(--side-w) minmax(0, 1fr); column-gap: var(--space-2xl); padding-block: var(--space-xl) var(--section-gap-s); }
.side { order: -1; position: sticky; top: calc(var(--nav-h) + var(--space-xl)); align-self: start; max-height: calc(100svh - var(--nav-h) - var(--space-3xl)); overflow: auto; padding-right: var(--space-xs); font-size: var(--text-sm); scrollbar-width: thin; }
.side h2 { max-width: none; margin: var(--space-lg) 0 var(--space-2xs); color: var(--color-muted); font-size: var(--text-sm); font-weight: 500; letter-spacing: 0; line-height: 1.4; }
.side h2:first-child { margin-top: 0; }
.side a { display: block; padding: var(--space-2xs) var(--space-sm); border-radius: var(--radius-input); color: var(--color-ink-2); text-decoration: none; }
.side a:hover { background: var(--color-paper-2); color: var(--color-ink); }
.side a:active { background: var(--color-paper-3); }
.side a[aria-current] { background: var(--color-accent-wash); color: var(--color-accent-deep); font-weight: 500; }
.article { min-width: 0; max-width: 58rem; }
.article > .lead { margin-top: var(--space-md); }
.article > p:not([class]) { margin-top: var(--space-md); }
.article h2 { font-size: var(--text-2xl); line-height: 1.15; margin: var(--space-2xl) 0 var(--space-sm); }
.article h3 { font-size: var(--text-lg); margin: var(--space-xl) 0 var(--space-xs); }
.article li { max-width: 70ch; }
.article ul, .article ol { padding-left: var(--space-lg); }
.article li + li { margin-top: var(--space-2xs); }
.crumbs { margin: 0 0 var(--space-sm); color: var(--color-muted); font-size: var(--text-sm); }
.crumbs a { color: var(--color-muted); }
.facts-row { display: flex; flex-wrap: wrap; gap: var(--space-xs) var(--space-lg); margin: var(--space-lg) 0 var(--space-sm); padding-block: var(--space-sm); border-block: 1px solid var(--color-rule); color: var(--color-ink-2); font-size: var(--text-sm); }
.endpoint { padding-block: var(--space-md) var(--space-2xl); border-bottom: 1px solid var(--color-rule); }
.endpoint > h2 { margin-top: var(--space-lg); }
.endpoint > p { margin-top: var(--space-sm); }
.route { display: inline-flex; flex-wrap: wrap; gap: var(--space-2xs) var(--space-sm); margin: var(--space-xs) 0 var(--space-sm); padding: var(--space-xs) var(--space-md); border: 1px solid var(--color-rule); border-radius: var(--radius-input); background: var(--color-paper-2); font: var(--text-sm)/1.5 var(--font-outlier); overflow-wrap: anywhere; }
.method { color: var(--color-success); }
.note { color: var(--color-ink-2); font-size: var(--text-sm); }
.codebox + .note { margin-top: var(--space-sm); }
table.params { width: 100%; border-collapse: collapse; margin: var(--space-lg) 0 var(--space-xs); font-size: var(--text-sm); }
table.params th, table.params td, .prose th, .prose td { text-align: left; vertical-align: top; padding: var(--space-sm) var(--space-md) var(--space-sm) 0; border-top: 1px solid var(--color-rule); }
table.params thead th, .prose thead th { border-top: 0; padding-top: 0; color: var(--color-muted); font: 500 var(--text-sm)/1.4 var(--font-display); white-space: nowrap; }
table.params tbody th { font-weight: 400; white-space: nowrap; width: 1%; }
table.params small { display: block; margin-top: var(--space-3xs); color: var(--color-muted); font-size: var(--text-xs); }
.label, .panel-label { margin: var(--space-lg) 0 var(--space-xs); color: var(--color-muted); font: 500 var(--text-sm)/1.4 var(--font-display); }
.tabs { display: none; gap: var(--space-2xs); width: max-content; max-width: 100%; overflow-x: auto; margin: var(--space-lg) 0 var(--space-sm); padding: var(--space-2xs); border-radius: var(--radius-input); background: var(--color-paper-3); }
.js .tabs { display: flex; }
.tabs button { min-height: 36px; padding: 0 var(--space-md); border: 1px solid transparent; border-radius: 4px; background: none; color: var(--color-ink-2); font: 500 var(--text-sm)/1 var(--font-display); white-space: nowrap; cursor: pointer; transition: background-color var(--dur-micro) var(--ease-out), color var(--dur-micro) var(--ease-out); }
.tabs button:hover { color: var(--color-ink); }
.tabs button:active { transform: translateY(1px); }
.tabs button[aria-selected=true] { border-color: var(--color-rule-2); background: var(--color-key-face); color: var(--color-ink); box-shadow: var(--shadow-card); }
.panel { margin-top: var(--space-sm); }
.js .panel { display: none; margin-top: 0; }
.js .panel.on { display: block; }
.js .panel-label { display: none; }
.preview { display: grid; place-items: center; padding: var(--space-lg); border: 1px solid var(--color-rule); border-radius: var(--radius-card); background: var(--color-paper-2); }
.preview img { max-height: 320px; width: auto; }
.sources { margin-top: var(--space-xl); }
.sources p { color: var(--color-ink-2); font-size: var(--text-sm); margin-top: var(--space-xs); }
.aside { margin-top: var(--space-xl); padding: var(--space-md) var(--space-lg); border-radius: var(--radius-card); background: var(--color-accent-wash); color: var(--color-ink); }

.wide { max-width: none; }
.measure > * { max-width: 70ch; }
.clients { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: var(--space-xl) var(--gap-columns); margin-block: var(--space-xl) var(--space-2xl); }
.client { display: grid; align-content: start; justify-items: start; gap: var(--space-sm); padding-top: var(--space-lg); border-top: 1px solid var(--color-rule-2); }
.client > * { max-width: 100%; }
.client .codebox { justify-self: stretch; }
.client h3 { margin: 0; }
.client p { color: var(--color-ink-2); }

.rows { margin: 0; padding: 0; list-style: none; border-bottom: 1px solid var(--color-rule); }
.rows a { display: grid; grid-template-columns: minmax(0, 5fr) minmax(0, 7fr); gap: var(--gap-columns); align-items: baseline; padding-block: var(--space-lg); border-top: 1px solid var(--color-rule); color: var(--color-ink); text-decoration: none; }
.rows a:hover .t { color: var(--color-accent); }
.rows .t { transition: color var(--dur-short) var(--ease-out); }
.rows span { color: var(--color-ink-2); }

.prose { max-width: 60rem; margin-inline: auto; }
.article.prose > * { max-width: 36rem; margin-left: max(0px, calc((100% - 36rem) / 2)); }
.article.prose > h1 { max-width: 36rem; }
.article.prose > .codebox, .article.prose > pre, .article.prose > table { max-width: none; margin-left: 0; }
.prose p, .prose li { line-height: 1.7; }
.prose > p, .prose > ul, .prose > ol, .prose > table, .prose > .codebox, .prose > pre { margin-top: var(--space-md); }
.prose > h1 + p { margin-top: var(--space-md); }
.prose table { width: 100%; border-collapse: collapse; font-size: var(--text-sm); }

/* the colophon */
.foot { margin-top: var(--section-gap-s); padding-block: var(--space-2xl) var(--space-xl); border-top: 1px solid var(--color-rule); background: var(--color-paper-2); color: var(--color-ink-2); font-size: var(--text-sm); }
.foot-grid { display: grid; grid-template-columns: repeat(12, minmax(0, 1fr)); column-gap: var(--gap-columns); row-gap: var(--space-xl); }
.colophon { grid-column: 1 / 6; display: grid; gap: var(--space-sm); align-content: start; }
.colophon .brand { font-size: var(--text-2xl); }
.colophon .brand svg { width: 32px; height: 32px; }
.colophon p { max-width: 44ch; }
.links { grid-column: 7 / 13; display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); column-gap: var(--gap-columns); }
.links h2 { max-width: none; margin-bottom: var(--space-xs); color: var(--color-ink); font-size: var(--text-sm); font-weight: 500; letter-spacing: 0; line-height: 1.4; }
.links a { display: block; padding-block: var(--space-2xs); color: var(--color-ink-2); text-decoration: none; }
.links a:hover { color: var(--color-accent-deep); text-decoration: underline; text-decoration-color: var(--color-accent); }
.fine { grid-column: 1 / -1; padding-top: var(--space-md); border-top: 1px solid var(--color-rule-2); color: var(--color-muted); }

@media (max-width: 60rem) {
  .hero { grid-template-columns: minmax(0, 1fr); gap: var(--space-xl); min-height: 0; padding-block: var(--space-xl) var(--space-2xl); }
  .hero h1 { max-width: 14ch; }
  .console pre.sample { max-height: calc(13lh + var(--space-md)); }
  .facts { grid-template-columns: minmax(0, 1fr); padding-block: var(--space-md); }
  .facts > div { padding: var(--space-md) 0; border-left: 0; border-top: 1px solid var(--color-rule); }
  .facts > div:first-child { border-top: 0; }
  .head-split h2, .head-split .lead, .cat h3, .cat ul, .split-text, .split-visual, .faqs h2, .faq-list, .dircat > h2, .dirs, .colophon, .links { grid-column: 1 / -1; }
  .cat h3, .faqs h2, .dircat > h2 { position: static; }
  .cat h3, .dircat > h2 { padding-bottom: var(--space-md); }
  .guides { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .docs { grid-template-columns: minmax(0, 1fr); row-gap: var(--space-2xl); }
  .side { order: 0; position: static; max-height: none; overflow: visible; padding-top: var(--space-xl); border-top: 1px solid var(--color-rule-2); }
  .clients { grid-template-columns: minmax(0, 1fr); }
}
@media (max-width: 40rem) {
  .bar { gap: var(--space-xs); padding-inline: var(--space-sm) var(--space-2xs); }
  .bar .brand { font-size: var(--text-md); }
  .chips { flex-wrap: nowrap; overflow-x: auto; scrollbar-width: none; margin-inline: calc(-1 * var(--gutter)); padding-inline: var(--gutter); }
  .chips a { flex-shrink: 0; }
  .bar nav a { padding-inline: var(--space-xs); }
  .cat ul, .guides { grid-template-columns: minmax(0, 1fr); }
  .cat li:nth-child(2) a { border-top-color: var(--color-rule); }
  .dir li, .rows a { grid-template-columns: minmax(0, 1fr); gap: var(--space-2xs); }
  .links { grid-template-columns: repeat(2, minmax(0, 1fr)); row-gap: var(--space-lg); }
  .strip { padding-left: var(--space-md); }
  table.params tbody th { white-space: normal; width: auto; }
  pre { padding-inline: var(--space-md); }
}
@media (max-width: 22rem) {
  .bar .brand span { position: absolute; width: 1px; height: 1px; overflow: hidden; clip-path: inset(50%); white-space: nowrap; }
}
`

const siteCSS = cssStamp + fontFaces + cssTokens + cssBase + cssPage

// siteJS adds code tabs (the chosen language is remembered in this browser only), a
// strip with a copy button over every code block, and colour for JSON answers. Without
// it every code sample is simply shown.
const siteJS = `
(function () {
  var key = "india-api-lang", chosen = null;
  try { chosen = localStorage.getItem(key); } catch (e) {}
  function pick(lang) {
    document.querySelectorAll(".example").forEach(function (example) {
      var panels = example.querySelectorAll(".panel");
      var use = example.querySelector('.panel[data-lang="' + lang + '"]') ? lang : panels[0].getAttribute("data-lang");
      panels.forEach(function (panel) { panel.classList.toggle("on", panel.getAttribute("data-lang") === use); });
      example.querySelectorAll(".tabs button").forEach(function (button) {
        button.setAttribute("aria-selected", button.getAttribute("data-lang") === use ? "true" : "false");
      });
    });
  }
  function escape(text) { return text.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;"); }
  document.querySelectorAll("pre.sample code").forEach(function (code) {
    var text = code.textContent, out = "", last = 0, match;
    var token = /("(?:\\.|[^"\\])*")(\s*:)?|\b(?:true|false|null)\b|-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?/g;
    while ((match = token.exec(text))) {
      out += escape(text.slice(last, match.index));
      if (match[2]) out += escape(match[0]);
      else out += '<span class="' + (match[1] ? "j-s" : "j-n") + '">' + escape(match[0]) + "</span>";
      last = token.lastIndex;
    }
    code.innerHTML = out + escape(text.slice(last));
  });
  document.querySelectorAll(".prose > pre").forEach(function (pre) {
    var box = document.createElement("div"), language = pre.querySelector("code[class*=language-]");
    box.className = "codebox";
    if (language) box.setAttribute("data-label", language.className.replace(/.*language-(\S+).*/, "$1"));
    pre.parentNode.insertBefore(box, pre);
    box.appendChild(pre);
  });
  document.querySelectorAll(".codebox").forEach(function (box) {
    var strip = document.createElement("div"), label = document.createElement("span"), button = document.createElement("button");
    strip.className = "strip";
    label.textContent = box.getAttribute("data-label") || "";
    button.type = "button";
    button.className = "copy";
    button.textContent = "Copy";
    strip.appendChild(label);
    strip.appendChild(button);
    box.insertBefore(strip, box.firstChild);
  });
  document.addEventListener("click", function (event) {
    var tab = event.target.closest(".tabs button");
    if (tab) {
      pick(tab.getAttribute("data-lang"));
      try { localStorage.setItem(key, tab.getAttribute("data-lang")); } catch (e) {}
    }
    var copy = event.target.closest(".copy");
    if (copy && navigator.clipboard) {
      navigator.clipboard.writeText(copy.closest(".codebox").querySelector("pre").innerText).then(function () {
        copy.textContent = "Copied";
        setTimeout(function () { copy.textContent = "Copy"; }, 1500);
      });
    }
  });
  pick(chosen || "curl");
})();
`

const brandMark = `<svg viewBox="0 0 64 64" aria-hidden="true"><rect width="64" height="64" rx="14"/><path d="M27 18c-5 0-7 2-7 7v3c0 3-1 4-4 4 3 0 4 1 4 4v3c0 5 2 7 7 7M37 18c5 0 7 2 7 7v3c0 3 1 4 4 4-3 0-4 1-4 4v3c0 5-2 7-7 7" fill="none" stroke-width="4" stroke-linecap="round" stroke-linejoin="round"/></svg>`

const htmlLayout = `
{{define "top"}}<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}}</title>
<meta name="description" content="{{.Description}}">
<link rel="canonical" href="{{.Canonical}}">
<meta name="theme-color" content="#f8fbfe">
<meta name="author" content="` + authorName + `">
<meta property="og:type" content="website">
<meta property="og:site_name" content="` + productName + `">
<meta property="og:locale" content="en_IN">
<meta property="og:title" content="{{.Title}}">
<meta property="og:description" content="{{.Description}}">
<meta property="og:url" content="{{.Canonical}}">
<meta property="og:image" content="{{.OGImage}}">
<meta property="og:image:width" content="1200">
<meta property="og:image:height" content="630">
<meta name="twitter:card" content="summary_large_image">
<link rel="icon" href="/favicon.svg" type="image/svg+xml">
<link rel="preload" href="/fonts/familjen-grotesk-latin-600-normal.woff2" as="font" type="font/woff2" crossorigin>
<link rel="preload" href="/fonts/ibm-plex-sans-latin-400-normal.woff2" as="font" type="font/woff2" crossorigin>
{{if .Markdown}}<link rel="alternate" type="text/markdown" href="{{.Markdown}}">
{{end}}<link rel="alternate" type="text/plain" href="/llms.txt" title="llms.txt">
<link rel="service-desc" type="application/openapi+json" href="/openapi.json">
<script>document.documentElement.className = "js";</script>
<style>` + siteCSS + `</style>
{{if .JSONLD}}<script type="application/ld+json">{{.JSONLD}}</script>
{{end}}</head>
<body>
<a class="skip" href="#main">Skip to content</a>
<header class="top"><div class="container"><div class="bar">
<a class="brand" href="/" aria-label="` + productName + ` home">` + brandMark + `<span>` + productName + `</span></a>
<nav aria-label="Main">
<a href="/docs"{{if eq .Nav "docs"}} aria-current="page"{{end}}>Docs</a>
<a href="/guides"{{if eq .Nav "guides"}} aria-current="page"{{end}}>Guides</a>
<a href="/ai"{{if eq .Nav "ai"}} aria-current="page"{{end}}>AI</a>
<a href="` + repoURL + `">GitHub</a>
</nav>
</div></div></header>
<main id="main">
{{end}}

{{define "bottom"}}</main>
<footer class="foot"><div class="container foot-grid">
<div class="colophon"><a class="brand" href="/" aria-label="` + productName + ` home">` + brandMark + `<span>` + productName + `</span></a><p>` + siteTagline + `. Built by <a href="` + authorURL + `">` + authorName + `</a>.</p></div>
<div class="links">
<div><h2>Use it</h2><a href="/docs">API reference</a><a href="/guides">Guides</a><a href="/ai">AI tools</a><a href="/openapi.json">OpenAPI</a><a href="/llms.txt">llms.txt</a></div>
<div><h2>Trust</h2><a href="/privacy">Privacy</a><a href="/security">Security</a><a href="/about">About and licences</a><a href="/healthz">Status</a></div>
<div><h2>Open source</h2><a href="` + repoURL + `">Source code</a><a href="` + repoURL + `/issues">Report a problem</a><a href="` + repoURL + `/blob/main/LICENSE">AGPL-3.0 licence</a></div>
</div>
<p class="fine">Company logos and trademarks belong to their owners. Data belongs to the sources named on each API page.</p>
</div></footer>
<script>` + siteJS + `</script>
</body>
</html>
{{end}}
`
