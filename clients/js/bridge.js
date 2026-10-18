#!/usr/bin/env node
// India API db bridge: lets psql and ordinary Postgres drivers reach a throwaway database,
// which is served over a WebSocket. It listens on localhost and carries each connection
// to the API. MIT-0.
//
//   npx india-api db-bridge [--port 5432] [--url wss://api.dharun.dev/v1/db/ws]
//   psql "postgresql://USER:PASSWORD@localhost:5432/NAME?sslmode=disable"
import net from "node:net";

const args = process.argv.slice(2);
const option = (name, fallback) => {
  const i = args.indexOf("--" + name);
  return i >= 0 && args[i + 1] ? args[i + 1] : fallback;
};

if (args[0] !== "db-bridge") {
  console.error("usage: india-api db-bridge [--port 5432] [--url wss://api.dharun.dev/v1/db/ws]");
  process.exit(2);
}
if (typeof WebSocket === "undefined") {
  console.error("The db bridge needs Node.js 22 or newer (for its built-in WebSocket).");
  process.exit(1);
}
const port = Number(option("port", "5432"));
const url = option("url", "wss://api.dharun.dev/v1/db/ws");

const server = net.createServer((socket) => {
  const ws = new WebSocket(url);
  ws.binaryType = "arraybuffer";
  const pending = [];
  socket.on("data", (data) => (ws.readyState === 1 ? ws.send(data) : pending.push(data)));
  ws.onopen = () => { for (const data of pending.splice(0)) ws.send(data); };
  ws.onmessage = (event) => socket.write(Buffer.from(event.data));
  ws.onclose = () => socket.end();
  ws.onerror = () => socket.destroy();
  socket.on("close", () => ws.close());
  socket.on("error", () => ws.close());
});
server.listen(port, "127.0.0.1", () => {
  console.error(`db bridge: localhost:${port} -> ${url}`);
});
