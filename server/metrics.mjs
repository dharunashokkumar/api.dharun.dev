// Minimal Prometheus text-format exporter. Labels are a fixed, small set on purpose:
// per-symbol detail belongs in the request log, not in metric cardinality.

const DURATION_BUCKETS = [0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1];

export function createMetrics({ version, dataset }) {
  const startedAt = Date.now() / 1000;
  const requests = new Map();
  const durations = new Map();
  const logoResults = new Map();
  let rateLimited = 0;

  return {
    observeRequest(route, method, status, seconds) {
      increment(requests, `route="${route}",method="${method}",status="${status}"`);

      let histogram = durations.get(route);
      if (!histogram) {
        histogram = { buckets: new Array(DURATION_BUCKETS.length).fill(0), sum: 0, count: 0 };
        durations.set(route, histogram);
      }
      for (let i = 0; i < DURATION_BUCKETS.length; i += 1) {
        if (seconds <= DURATION_BUCKETS[i]) histogram.buckets[i] += 1;
      }
      histogram.sum += seconds;
      histogram.count += 1;
    },

    // result: found | fallback | not_found
    observeLogo(format, result) {
      increment(logoResults, `format="${format}",result="${result}"`);
    },

    observeRateLimited() {
      rateLimited += 1;
    },

    render() {
      const lines = [];
      const metric = (name, type, help) => lines.push(`# HELP ${name} ${help}`, `# TYPE ${name} ${type}`);

      metric("logos_build_info", "gauge", "Running version of the logos API.");
      lines.push(`logos_build_info{version="${version}"} 1`);

      metric("logos_dataset_logos", "gauge", "Logos in the loaded dataset, by exchange.");
      for (const [exchange, count] of Object.entries(dataset.counts?.by_exchange || {})) {
        lines.push(`logos_dataset_logos{exchange="${exchange}"} ${count}`);
      }

      metric("logos_dataset_generated_timestamp_seconds", "gauge", "When the company metadata was generated.");
      lines.push(`logos_dataset_generated_timestamp_seconds ${Date.parse(dataset.generatedAt) / 1000 || 0}`);

      metric("logos_http_requests_total", "counter", "HTTP requests by route, method and status.");
      for (const [labels, count] of requests) lines.push(`logos_http_requests_total{${labels}} ${count}`);

      metric("logos_http_request_duration_seconds", "histogram", "Time to produce a response, by route.");
      for (const [route, histogram] of durations) {
        DURATION_BUCKETS.forEach((le, i) => {
          lines.push(`logos_http_request_duration_seconds_bucket{route="${route}",le="${le}"} ${histogram.buckets[i]}`);
        });
        lines.push(`logos_http_request_duration_seconds_bucket{route="${route}",le="+Inf"} ${histogram.count}`);
        lines.push(`logos_http_request_duration_seconds_sum{route="${route}"} ${histogram.sum}`);
        lines.push(`logos_http_request_duration_seconds_count{route="${route}"} ${histogram.count}`);
      }

      metric("logos_logo_requests_total", "counter", "Logo lookups by output format and outcome.");
      for (const [labels, count] of logoResults) lines.push(`logos_logo_requests_total{${labels}} ${count}`);

      metric("logos_rate_limited_total", "counter", "Requests rejected by the rate limiter.");
      lines.push(`logos_rate_limited_total ${rateLimited}`);

      const memory = process.memoryUsage();
      metric("process_resident_memory_bytes", "gauge", "Resident memory size in bytes.");
      lines.push(`process_resident_memory_bytes ${memory.rss}`);
      metric("nodejs_heap_used_bytes", "gauge", "V8 heap in use in bytes.");
      lines.push(`nodejs_heap_used_bytes ${memory.heapUsed}`);
      metric("process_start_time_seconds", "gauge", "Start time of the process since unix epoch.");
      lines.push(`process_start_time_seconds ${startedAt}`);

      return `${lines.join("\n")}\n`;
    },
  };
}

function increment(map, key) {
  map.set(key, (map.get(key) || 0) + 1);
}
