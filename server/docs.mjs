import { PNG_DEFAULT_SIZE, PNG_MAX_SIZE, PNG_MIN_SIZE } from "./images.mjs";

const AI_CRAWLERS = [
  "GPTBot",
  "OAI-SearchBot",
  "ChatGPT-User",
  "ClaudeBot",
  "Claude-User",
  "Claude-SearchBot",
  "PerplexityBot",
  "Perplexity-User",
  "Google-Extended",
  "Applebot-Extended",
  "CCBot",
];

// "NSE:TCS" style reference: unambiguous across exchanges and readable in a URL.
export function symbolRef(logo) {
  return encodeURIComponent(logo.id).replace(/%3A/g, ":");
}

export function companyJson(logo, urls) {
  const ref = symbolRef(logo);
  return {
    id: logo.id,
    exchange: logo.exchange,
    ticker: logo.ticker,
    company: logo.company,
    sector: logo.sector,
    industry: logo.industry,
    oneLine: logo.oneLine,
    isin: logo.isin,
    currency: logo.currency,
    marketCap: logo.marketCap,
    isIndexConstituent: logo.isIndexConstituent,
    indexes: logo.indexes || [],
    logo: {
      svg: `${urls.api}/logo/${ref}`,
      png: `${urls.api}/logo/${ref}?format=png&size=${PNG_DEFAULT_SIZE}`,
    },
    links: {
      self: `${urls.api}/companies/${ref}`,
      page: `${urls.site}/company/${ref}`,
    },
  };
}

export function metaJson(dataset, urls, version) {
  return {
    name: "Indian Listed Company Logos API",
    version,
    generatedAt: dataset.generatedAt,
    counts: { total: dataset.logos.length, ...dataset.counts?.by_exchange },
    endpoints: {
      logo: `${urls.api}/logo/{symbol}`,
      company: `${urls.api}/companies/{symbol}`,
      search: `${urls.api}/search?q={query}`,
      mcp: `${urls.site}/mcp`,
      openapi: `${urls.site}/openapi.json`,
      llms: `${urls.site}/llms.txt`,
    },
    sectors: dataset.sectors,
    indexes: dataset.indexes,
  };
}

export function rootIndexJson(urls) {
  return {
    name: new URL(urls.origin).host,
    apis: [
      {
        name: "logos",
        description: "SVG and PNG logos plus metadata for NSE and BSE listed companies.",
        url: `${urls.site}/`,
        openapi: `${urls.site}/openapi.json`,
        llms: `${urls.site}/llms.txt`,
      },
    ],
  };
}

export function robotsTxt(urls) {
  const lines = ["User-agent: *", "Allow: /", ""];
  for (const agent of AI_CRAWLERS) lines.push(`User-agent: ${agent}`, "Allow: /", "");
  lines.push(`Sitemap: ${urls.site}/sitemap.xml`, "");
  return lines.join("\n");
}

export function sitemapXml(dataset, urls) {
  const lastmod = String(dataset.generatedAt || "").slice(0, 10);
  const entry = (loc) =>
    `  <url><loc>${escapeXml(loc)}</loc>${lastmod ? `<lastmod>${lastmod}</lastmod>` : ""}</url>`;

  return [
    '<?xml version="1.0" encoding="UTF-8"?>',
    '<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">',
    entry(`${urls.site}/`),
    ...dataset.logos.map((logo) => entry(`${urls.site}/company/${symbolRef(logo)}`)),
    "</urlset>",
    "",
  ].join("\n");
}

export function llmsTxt(dataset, urls) {
  return `# Indian Listed Company Logos API

> Free logo API for companies listed on India's NSE and BSE stock exchanges. ${dataset.logos.length} logos as SVG or PNG, with company metadata. No API key, no sign-up, CORS enabled.

To show the logo of an Indian listed company, use its ticker in this URL:

    ${urls.api}/logo/{symbol}

Example: \`<img src="${urls.api}/logo/TCS" alt="TCS logo">\`

## Symbols

Any of these forms work wherever a symbol is accepted:

- Ticker: \`TCS\`, \`RELIANCE\`, \`M&M\` (URL-encode \`&\` as \`%26\`; \`MM\` also works)
- Exchange-qualified: \`NSE:TCS\`, \`BSE:AAIL\`, \`NSE_TCS\`
- Yahoo Finance style: \`TCS.NS\`, \`AAIL.BO\`
- ISIN: \`INE467B01029\`

A bare ticker prefers NSE. Add \`?exchange=nse\` or \`?exchange=bse\` to force one.

## Endpoints

- [Logo image](${urls.api}/logo/TCS): \`GET /v1/logo/{symbol}\` returns the image. Query: \`format=svg|png\` (default svg), \`size=${PNG_MIN_SIZE}-${PNG_MAX_SIZE}\` (PNG pixels, default ${PNG_DEFAULT_SIZE}), \`fallback=monogram\` (return an initials tile instead of 404). A \`.svg\` or \`.png\` suffix on the symbol also selects the format.
- [Company metadata](${urls.api}/companies/TCS): \`GET /v1/companies/{symbol}\` returns JSON with name, sector, industry, ISIN, market cap, index memberships and logo URLs.
- [Search](${urls.api}/search?q=tata): \`GET /v1/search?q=\` searches ticker, company name, sector, industry, ISIN and index names. Query: \`exchange\`, \`sector\`, \`index\`, \`limit\` (max 100), \`offset\`.
- [Dataset summary](${urls.api}): \`GET /v1\` lists counts, sectors and index names.
- [OpenAPI description](${urls.site}/openapi.json)
- [MCP server](${urls.site}/mcp): Streamable HTTP, tools \`search_logos\` and \`get_logo\`.

## Notes

- Logos are square and vector; SVG scales to any size.
- Unknown symbols return HTTP 404 with a JSON error unless \`fallback=monogram\` is set.
- Company logos and trademarks belong to their respective owners.
- Human-browsable directory: ${urls.site}/
`;
}

export function openApiJson(dataset, urls, version) {
  const symbolParam = {
    name: "symbol",
    in: "path",
    required: true,
    description:
      "Ticker (TCS), exchange-qualified ticker (NSE:TCS, NSE_TCS), Yahoo style (TCS.NS, AAIL.BO) or ISIN (INE467B01029).",
    schema: { type: "string" },
    example: "TCS",
  };
  const exchangeParam = {
    name: "exchange",
    in: "query",
    description: "Force an exchange. A bare ticker prefers NSE.",
    schema: { type: "string", enum: ["nse", "bse"] },
  };
  const errorResponse = {
    description: "Error",
    content: { "application/json": { schema: { $ref: "#/components/schemas/Error" } } },
  };

  return {
    openapi: "3.1.0",
    info: {
      title: "Indian Listed Company Logos API",
      version,
      description: `SVG and PNG logos plus metadata for ${dataset.logos.length} companies listed on NSE and BSE. No API key required.`,
    },
    servers: [{ url: urls.api }],
    paths: {
      "/logo/{symbol}": {
        get: {
          operationId: "getLogo",
          summary: "Get a company logo image",
          parameters: [
            symbolParam,
            exchangeParam,
            { name: "format", in: "query", schema: { type: "string", enum: ["svg", "png"], default: "svg" } },
            {
              name: "size",
              in: "query",
              description: "PNG width and height in pixels.",
              schema: { type: "integer", minimum: PNG_MIN_SIZE, maximum: PNG_MAX_SIZE, default: PNG_DEFAULT_SIZE },
            },
            {
              name: "fallback",
              in: "query",
              description: "Return a generated initials tile instead of 404 when the symbol is unknown.",
              schema: { type: "string", enum: ["monogram"] },
            },
          ],
          responses: {
            200: {
              description: "The logo",
              content: { "image/svg+xml": {}, "image/png": {} },
            },
            400: errorResponse,
            404: errorResponse,
          },
        },
      },
      "/companies/{symbol}": {
        get: {
          operationId: "getCompany",
          summary: "Get company metadata and logo URLs",
          parameters: [symbolParam, exchangeParam],
          responses: {
            200: {
              description: "The company",
              content: { "application/json": { schema: { $ref: "#/components/schemas/Company" } } },
            },
            404: errorResponse,
          },
        },
      },
      "/search": {
        get: {
          operationId: "searchCompanies",
          summary: "Search companies",
          parameters: [
            { name: "q", in: "query", description: "Ticker, company name, sector, industry, ISIN or index name.", schema: { type: "string" } },
            exchangeParam,
            { name: "sector", in: "query", schema: { type: "string" } },
            { name: "index", in: "query", description: "Index name, for example Nifty 50.", schema: { type: "string" } },
            { name: "limit", in: "query", schema: { type: "integer", minimum: 1, maximum: 100, default: 20 } },
            { name: "offset", in: "query", schema: { type: "integer", minimum: 0, default: 0 } },
          ],
          responses: {
            200: {
              description: "Matching companies, best match first",
              content: {
                "application/json": {
                  schema: {
                    type: "object",
                    properties: {
                      total: { type: "integer" },
                      limit: { type: "integer" },
                      offset: { type: "integer" },
                      results: { type: "array", items: { $ref: "#/components/schemas/Company" } },
                    },
                  },
                },
              },
            },
            400: errorResponse,
          },
        },
      },
    },
    components: {
      schemas: {
        Company: {
          type: "object",
          properties: {
            id: { type: "string", example: "NSE:TCS" },
            exchange: { type: "string", enum: ["NSE", "BSE"] },
            ticker: { type: "string" },
            company: { type: "string" },
            sector: { type: "string" },
            industry: { type: "string" },
            oneLine: { type: "string" },
            isin: { type: "string" },
            currency: { type: "string" },
            marketCap: { type: ["number", "null"] },
            isIndexConstituent: { type: "boolean" },
            indexes: { type: "array", items: { type: "string" } },
            logo: {
              type: "object",
              properties: { svg: { type: "string", format: "uri" }, png: { type: "string", format: "uri" } },
            },
            links: {
              type: "object",
              properties: { self: { type: "string", format: "uri" }, page: { type: "string", format: "uri" } },
            },
          },
        },
        Error: {
          type: "object",
          properties: {
            error: {
              type: "object",
              properties: { code: { type: "string" }, message: { type: "string" } },
            },
          },
        },
      },
    },
  };
}

// One crawlable page per company: what search engines and AI search actually land on.
export function companyPageHtml(logo, urls) {
  const company = companyJson(logo, urls);
  const name = logo.company || logo.ticker;
  const title = `${name} logo (${logo.exchange}: ${logo.ticker}) - SVG and PNG`;
  const description = `Download or embed the ${name} logo as SVG or PNG. ${logo.oneLine || ""}`.trim();
  const facts = [
    ["Exchange", logo.exchange],
    ["Ticker", logo.ticker],
    ["ISIN", logo.isin],
    ["Sector", logo.sector],
    ["Industry", logo.industry],
    ["Indexes", (logo.indexes || []).slice(0, 8).join(", ")],
  ].filter(([, value]) => value);

  const jsonLd = {
    "@context": "https://schema.org",
    "@type": "ImageObject",
    name: `${name} logo`,
    contentUrl: company.logo.svg,
    encodingFormat: "image/svg+xml",
    about: { "@type": "Corporation", name, tickerSymbol: `${logo.exchange}:${logo.ticker}` },
  };

  return `<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>${escapeHtml(title)}</title>
    <meta name="description" content="${escapeHtml(description)}" />
    <link rel="canonical" href="${escapeHtml(company.links.page)}" />
    <meta property="og:title" content="${escapeHtml(title)}" />
    <meta property="og:description" content="${escapeHtml(description)}" />
    <meta property="og:image" content="${escapeHtml(`${company.logo.svg}?format=png&size=512`)}" />
    <link rel="icon" href="${escapeHtml(`${urls.site}/favicon.svg`)}" type="image/svg+xml" />
    <script type="application/ld+json">${JSON.stringify(jsonLd).replace(/</g, "\\u003c")}</script>
    <style>
      body { margin: 0; font: 16px/1.5 system-ui, sans-serif; color: #171512; background: #f3efe7; }
      main { max-width: 720px; margin: 0 auto; padding: 32px 16px 64px; }
      a { color: #2248ff; }
      .logo { width: 128px; height: 128px; border: 1px solid #252018; background: #fffaf0; }
      h1 { margin: 24px 0 4px; font-size: 28px; line-height: 1.2; }
      dl { display: grid; grid-template-columns: max-content 1fr; gap: 6px 16px; margin: 24px 0; }
      dt { color: #4c463d; }
      dd { margin: 0; }
      pre { overflow-x: auto; padding: 12px; background: #fffaf0; border: 1px solid #252018; font-size: 14px; }
      footer { margin-top: 40px; color: #4c463d; font-size: 14px; }
    </style>
  </head>
  <body>
    <main>
      <p><a href="${escapeHtml(`${urls.site}/`)}">Indian Listed Company Logos</a></p>
      <img class="logo" src="${escapeHtml(company.logo.svg)}" alt="${escapeHtml(name)} logo" width="128" height="128" />
      <h1>${escapeHtml(name)} logo</h1>
      <p>${escapeHtml(logo.oneLine || "")}</p>
      <dl>
${facts.map(([label, value]) => `        <dt>${escapeHtml(label)}</dt><dd>${escapeHtml(value)}</dd>`).join("\n")}
      </dl>
      <h2>Use this logo</h2>
      <pre>${escapeHtml(`SVG   ${company.logo.svg}\nPNG   ${company.logo.png}\nJSON  ${company.links.self}`)}</pre>
      <pre>${escapeHtml(`<img src="${company.logo.svg}" alt="${name} logo" width="64" height="64">`)}</pre>
      <footer>
        Logos and trademarks belong to their respective owners.
        API docs: <a href="${escapeHtml(`${urls.site}/llms.txt`)}">llms.txt</a>,
        <a href="${escapeHtml(`${urls.site}/openapi.json`)}">OpenAPI</a>.
      </footer>
    </main>
  </body>
</html>
`;
}

function escapeHtml(value) {
  return String(value ?? "")
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#39;");
}

function escapeXml(value) {
  return escapeHtml(value);
}
