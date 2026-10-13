import { BadRequestError } from "./dataset.mjs";
import { companyJson } from "./docs.mjs";

// Stateless Model Context Protocol server over Streamable HTTP: every POST carries one
// JSON-RPC message and gets one JSON response. No sessions, no server-initiated stream.

const PROTOCOL_VERSIONS = ["2025-11-25", "2025-06-18", "2025-03-26", "2024-11-05"];
const DEFAULT_PROTOCOL_VERSION = "2025-06-18";

const TOOLS = [
  {
    name: "search_logos",
    description:
      "Search Indian listed companies (NSE and BSE) by ticker, company name, sector, industry, ISIN or index name. Returns company metadata with ready-to-use SVG and PNG logo URLs.",
    inputSchema: {
      type: "object",
      properties: {
        query: { type: "string", description: "Search text, for example 'tata', 'HDFCBANK' or 'Nifty 50'." },
        exchange: { type: "string", enum: ["NSE", "BSE"], description: "Limit results to one exchange." },
        limit: { type: "integer", minimum: 1, maximum: 25, default: 10 },
      },
      required: ["query"],
    },
  },
  {
    name: "get_logo",
    description:
      "Get the logo URLs and metadata for one Indian listed company. Accepts a ticker (TCS), an exchange-qualified ticker (NSE:TCS), a Yahoo Finance symbol (TCS.NS) or an ISIN.",
    inputSchema: {
      type: "object",
      properties: {
        symbol: { type: "string", description: "Ticker, exchange-qualified ticker, Yahoo symbol or ISIN." },
        exchange: { type: "string", enum: ["NSE", "BSE"], description: "Force an exchange. A bare ticker prefers NSE." },
        include_svg: { type: "boolean", default: false, description: "Also return the SVG markup itself." },
      },
      required: ["symbol"],
    },
  },
];

export function createMcpHandler({ dataset, version, readSvg }) {
  async function callTool(name, args, urls) {
    if (name === "search_logos") {
      if (typeof args.query !== "string" || !args.query.trim()) throw new BadRequestError("query is required.");
      const limit = Math.min(Math.max(Number.parseInt(args.limit, 10) || 10, 1), 25);
      const { total, results } = dataset.search({ q: args.query, exchange: args.exchange, limit });
      return { total, results: results.map((logo) => companyJson(logo, urls)) };
    }

    if (name === "get_logo") {
      if (typeof args.symbol !== "string" || !args.symbol.trim()) throw new BadRequestError("symbol is required.");
      const logo = dataset.resolve(args.symbol, args.exchange);
      if (!logo) throw new BadRequestError(`No logo found for "${args.symbol}". Try search_logos.`);
      const company = companyJson(logo, urls);
      if (args.include_svg) company.svg = (await readSvg(logo)).toString("utf8");
      return company;
    }

    return undefined;
  }

  // Returns the JSON-RPC response object, or null for notifications (which get no body).
  return async function handle(message, urls) {
    if (!message || typeof message !== "object" || Array.isArray(message) || message.jsonrpc !== "2.0") {
      return rpcError(null, -32600, "Expected a single JSON-RPC 2.0 message.");
    }
    if (typeof message.method !== "string") {
      return message.id === undefined ? null : rpcError(message.id, -32600, "Missing method.");
    }
    if (message.id === undefined || message.id === null) return null;

    const { id, method, params = {} } = message;

    if (method === "initialize") {
      const requested = params.protocolVersion;
      return rpcResult(id, {
        protocolVersion: PROTOCOL_VERSIONS.includes(requested) ? requested : DEFAULT_PROTOCOL_VERSION,
        capabilities: { tools: {} },
        serverInfo: { name: "indian-listed-company-logos", title: "Indian Listed Company Logos", version },
        instructions:
          "Find logos for companies listed on India's NSE and BSE exchanges. Use search_logos when you only know a company name, get_logo when you have a ticker or ISIN. The returned logo.svg URL can be embedded directly in an <img> tag.",
      });
    }
    if (method === "ping") return rpcResult(id, {});
    if (method === "tools/list") return rpcResult(id, { tools: TOOLS });

    if (method === "tools/call") {
      if (!TOOLS.some((tool) => tool.name === params.name)) {
        return rpcError(id, -32602, `Unknown tool: ${params.name}`);
      }
      try {
        const result = await callTool(params.name, params.arguments || {}, urls);
        return rpcResult(id, {
          content: [{ type: "text", text: JSON.stringify(result, null, 2) }],
          structuredContent: result,
        });
      } catch (error) {
        if (!(error instanceof BadRequestError)) throw error;
        return rpcResult(id, { content: [{ type: "text", text: error.message }], isError: true });
      }
    }

    return rpcError(id, -32601, `Method not found: ${method}`);
  };
}

export function rpcError(id, code, message) {
  return { jsonrpc: "2.0", id, error: { code, message } };
}

function rpcResult(id, result) {
  return { jsonrpc: "2.0", id, result };
}
