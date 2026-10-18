//go:build linux

// Command sandboxd is the sandbox controller. It runs as root in the sandbox VM (it starts
// the jailer, which drops each microVM to a user of its own), listens on the private link
// for the API, and serves metrics for Prometheus on a second port. Settings come from the
// environment (deploy/sandbox/sandboxd.env).
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/dharunashokkumar/api.dharun.dev/internal/sandboxd"
)

var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, `{"msg":"fatal","error":%q}`+"\n", err.Error())
		os.Exit(1)
	}
}

func run() error {
	secret := strings.TrimSpace(os.Getenv("SANDBOX_SECRET"))
	if file := os.Getenv("SANDBOX_SECRET_FILE"); secret == "" && file != "" {
		raw, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		secret = strings.TrimSpace(string(raw))
	}
	if len(secret) < 32 {
		return errors.New("SANDBOX_SECRET (or SANDBOX_SECRET_FILE) must hold at least 32 characters")
	}
	base := env("SANDBOX_HOME", "/opt/sandbox")
	cfg := sandboxd.Config{
		Version:         version,
		Firecracker:     env("FIRECRACKER", base+"/bin/firecracker"),
		Jailer:          env("JAILER", base+"/bin/jailer"),
		Kernel:          env("KERNEL", base+"/vmlinux"),
		Images:          env("IMAGES", base+"/images"),
		Scratch:         env("SCRATCH", base+"/images/scratch.ext4"),
		JailRoot:        env("JAIL_ROOT", base+"/jail"),
		StateDir:        env("STATE_DIR", "/var/lib/sandboxd"),
		UIDBase:         envInt("UID_BASE", 61000),
		UIDCount:        envInt("UID_COUNT", 1000),
		MemoryCapMB:     envInt("MEMORY_CAP_MB", 2560),
		CodeMemoryMB:    envInt("CODE_MEMORY_MB", 256),
		BrowserMemoryMB: envInt("BROWSER_MEMORY_MB", 1024),
		Warm:            envInt("WARM", 2),
		BootTimeout:     time.Duration(envInt("BOOT_TIMEOUT_SECONDS", 10)) * time.Second,
		QueueWait:       time.Duration(envInt("QUEUE_WAIT_SECONDS", 10)) * time.Second,
		PauseFile:       env("PAUSE_FILE", "/etc/sandboxd/paused"),
		PerSite:         envInt("BROWSER_PER_SITE", 10),
		Postgres: sandboxd.PostgresConfig{
			Enabled: env("POSTGRES", "on") == "on",
			PSQL:    strings.Fields(env("PSQL", "runuser -u postgres -- psql")),
		},
	}
	controller, err := sandboxd.New(cfg)
	if err != nil {
		return err
	}
	defer controller.Close()

	public := &http.Server{Addr: env("LISTEN", "0.0.0.0:7070"), Handler: controller.Handler(secret), ReadHeaderTimeout: 10 * time.Second}
	metrics := &http.Server{Addr: env("METRICS_LISTEN", "0.0.0.0:9465"), Handler: controller.MetricsHandler(), ReadHeaderTimeout: 10 * time.Second}
	failed := make(chan error, 2)
	go func() { failed <- public.ListenAndServe() }()
	go func() { failed <- metrics.ListenAndServe() }()
	fmt.Fprintf(os.Stderr, `{"msg":"listening","version":%q,"address":%q,"metrics":%q}`+"\n", version, public.Addr, metrics.Addr)

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
	metrics.Shutdown(ctx)
	return nil
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
