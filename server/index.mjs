import http from "node:http";

import { createApp } from "./app.mjs";
import { loadConfig } from "./config.mjs";

const config = loadConfig();
const app = createApp(config);

const server = http.createServer(app.handler);
// Slow or idle clients must not pin sockets on a small box.
server.requestTimeout = 15_000;
server.headersTimeout = 10_000;
server.keepAliveTimeout = 65_000;

server.listen(config.port, config.host, () => {
  console.log(
    JSON.stringify({
      msg: "listening",
      version: config.version,
      address: `http://${config.host}:${config.port}${config.basePath}/`,
      logos: app.dataset.logos.length,
    }),
  );
});

let metricsServer;
if (config.metricsPort) {
  metricsServer = http.createServer(app.metricsHandler);
  metricsServer.listen(config.metricsPort, config.metricsHost, () => {
    console.log(JSON.stringify({ msg: "metrics", address: `http://${config.metricsHost}:${config.metricsPort}/metrics` }));
  });
}

for (const signal of ["SIGINT", "SIGTERM"]) {
  process.on(signal, () => {
    app.close();
    metricsServer?.close();
    server.close(() => process.exit(0));
    server.closeIdleConnections();
    setTimeout(() => process.exit(0), 5_000).unref();
  });
}
