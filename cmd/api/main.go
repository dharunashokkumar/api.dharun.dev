// Command api serves api.dharun.dev: free, open APIs for Indian developers.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
	// Time zones are built in, so the panchang works on a server without tzdata.
	_ "time/tzdata"

	"github.com/dharunashokkumar/api.dharun.dev/internal/api"
)

// version is set at build time with -ldflags "-X main.version=1.2.3".
var version = "dev"

func main() {
	if err := run(); err != nil {
		logLine(map[string]any{"msg": "fatal", "error": err.Error()})
		os.Exit(1)
	}
}

func run() error {
	host := env("HOST", "127.0.0.1")
	port := env("PORT", "8080")
	// Prometheus scrapes this listener directly; it is never reachable through the public route.
	metricsHost := env("METRICS_HOST", "127.0.0.1")
	metricsPort := env("METRICS_PORT", "9464")

	stateDir := os.Getenv("STATE_DIR")
	if stateDir != "" {
		if err := os.MkdirAll(stateDir, 0o755); err != nil {
			return err
		}
	}
	server, err := api.New(api.Config{
		Version: version,
		// SEED_DIR holds the datasets that came with the release (build them with: apidata
		// build). DATA_DIR holds the copies this server fetched itself; the newer one is served.
		SeedDir:            env("SEED_DIR", "seed"),
		DataDir:            os.Getenv("DATA_DIR"),
		ReloadEvery:        time.Duration(envInt("RELOAD_SECONDS", 15)) * time.Second,
		MaxBodyBytes:       envInt("MAX_BODY_KB", 256) << 10,
		HeavyConcurrency:   envInt("HEAVY_CONCURRENCY", 0),
		Resvg:              os.Getenv("RESVG"),
		RenderConcurrency:  envInt("RENDER_CONCURRENCY", 4),
		ModelConcurrency:   envInt("MODEL_CONCURRENCY", 1),
		TextModelMemory:    int64(envInt("TEXT_MODEL_MEMORY_MB", 2048)) << 20,
		TextModelIdle:      time.Duration(envInt("TEXT_MODEL_IDLE_MINUTES", 10)) * time.Minute,
		TextModelThreads:   envInt("TEXT_MODEL_THREADS", 4),
		Programs:           programPaths(),
		PublicOrigin:       strings.TrimRight(os.Getenv("PUBLIC_ORIGIN"), "/"),
		TrustProxy:         envBool("TRUST_PROXY", false),
		RateLimitPerMinute: envInt("RATE_LIMIT_PER_MIN", 600),
		LogRequests:        envBool("LOG_REQUESTS", true),
		PNGCacheBytes:      envInt("PNG_CACHE_MB", 32) * 1024 * 1024,
		StateDir:           stateDir,
		GitHubClientID:     os.Getenv("GITHUB_CLIENT_ID"),
		SandboxURL:         os.Getenv("SANDBOX_URL"),
		SandboxSecret:      os.Getenv("SANDBOX_SECRET"),
		SandboxPGAddr:      os.Getenv("SANDBOX_PG_ADDR"),
		TurnstileSiteKey:   os.Getenv("TURNSTILE_SITE_KEY"),
		TurnstileSecret:    os.Getenv("TURNSTILE_SECRET"),
	})
	if err != nil {
		return err
	}
	defer server.Close()

	// Slow or idle clients must not pin sockets on a small box.
	public := &http.Server{
		Addr:              net.JoinHostPort(host, port),
		Handler:           server,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      45 * time.Second,
		IdleTimeout:       65 * time.Second,
	}
	failed := make(chan error, 2)
	go func() { failed <- public.ListenAndServe() }()
	logLine(map[string]any{"msg": "listening", "version": version, "address": "http://" + public.Addr + "/"})

	var private *http.Server
	if metricsPort != "0" {
		private = &http.Server{Addr: net.JoinHostPort(metricsHost, metricsPort), Handler: server.MetricsHandler(), ReadHeaderTimeout: 10 * time.Second}
		go func() { failed <- private.ListenAndServe() }()
		logLine(map[string]any{"msg": "metrics", "address": "http://" + private.Addr + "/metrics"})
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	select {
	case err := <-failed:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-signals:
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	public.Shutdown(ctx)
	if private != nil {
		private.Shutdown(ctx)
	}
	return nil
}

// programPaths reads where outside programs are, for the ones set in the environment:
// TYPST=/path/to/typst, DEEP_FILTER=..., ONNX_RUN=... Unset ones are looked for next to
// the binary, then on PATH.
func programPaths() map[string]string {
	paths := map[string]string{}
	for _, name := range []string{"typst", "piper", "whisper-cli", "deep-filter", "onnx-run", "tesseract", "vips", "vipsheader", "ffmpeg", "pdftotext", "pdftoppm"} {
		if value := os.Getenv(strings.ToUpper(strings.ReplaceAll(name, "-", "_"))); value != "" {
			paths[name] = value
		}
	}
	return paths
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func envInt(name string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(name))
	if err != nil || value < 0 {
		return fallback
	}
	return value
}

func envBool(name string, fallback bool) bool {
	switch strings.ToLower(os.Getenv(name)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	return fallback
}

func logLine(fields map[string]any) {
	encoded, _ := json.Marshal(fields)
	fmt.Println(string(encoded))
}
