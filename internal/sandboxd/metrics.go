//go:build linux

package sandboxd

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// labelled is a counter or sum per label value.
type labelled struct {
	mu     sync.Mutex
	values map[string]float64
}

func (l *labelled) add(label string, value int64) { l.addf(label, float64(value)) }

func (l *labelled) addf(label string, value float64) {
	l.mu.Lock()
	if l.values == nil {
		l.values = map[string]float64{}
	}
	l.values[label] += value
	l.mu.Unlock()
}

func (l *labelled) write(b *strings.Builder, name, label, help, kind string) {
	fmt.Fprintf(b, "# HELP %s %s\n# TYPE %s %s\n", name, help, name, kind)
	l.mu.Lock()
	defer l.mu.Unlock()
	keys := make([]string, 0, len(l.values))
	for key := range l.values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if label == "" {
			fmt.Fprintf(b, "%s %g\n", name, l.values[key])
		} else {
			fmt.Fprintf(b, "%s{%s=%q} %g\n", name, label, key, l.values[key])
		}
	}
}

// metrics are what Prometheus scrapes from the controller. Nothing in them names an
// account or what ran.
type metrics struct {
	vmStarts     labelled // by image and outcome, "code/ok"
	bootSeconds  labelled // by image: sum
	bootCount    labelled // by image
	runs         labelled // by outcome: ok, error, time, memory, output
	runSeconds   labelled // "" sum of wall time
	cpuSeconds   labelled // by kind: run, session, browser
	limits       labelled // by limit
	cpuBound     labelled // "" runs that hit the time limit with the CPU busy throughout
	busy         labelled // by what was refused for want of a slot
	queueWait    labelled // "" sum of seconds waited for a slot
	queueCount   labelled
	proxyBytes   labelled // by policy
	proxyRefused labelled // by policy
	browser      labelled // by action/outcome
	sessions     labelled // started, by image
	databases    labelled // by outcome: created, dropped, expired, full
}

func (c *Controller) renderMetrics() string {
	var b strings.Builder
	m := c.metrics
	m.vmStarts.write(&b, "sandbox_vm_starts_total", "image_outcome", "MicroVMs started, by image and outcome.", "counter")
	m.bootSeconds.write(&b, "sandbox_vm_boot_seconds_sum", "image", "Time from start to the agent answering.", "counter")
	m.bootCount.write(&b, "sandbox_vm_boot_seconds_count", "image", "MicroVMs booted.", "counter")
	m.runs.write(&b, "sandbox_runs_total", "outcome", "Run-once requests by outcome.", "counter")
	m.runSeconds.write(&b, "sandbox_run_seconds_sum", "", "Wall time of runs.", "counter")
	m.cpuSeconds.write(&b, "sandbox_cpu_seconds_total", "kind", "Processor time used by microVMs.", "counter")
	m.limits.write(&b, "sandbox_limits_hit_total", "limit", "Limits runs and commands ran into.", "counter")
	m.cpuBound.write(&b, "sandbox_cpu_bound_timeouts_total", "", "Runs that used the processor flat out until the time limit (a mining pattern).", "counter")
	m.busy.write(&b, "sandbox_busy_total", "what", "Requests refused because no microVM could be started in time.", "counter")
	m.queueWait.write(&b, "sandbox_queue_wait_seconds_sum", "", "Time requests waited for memory.", "counter")
	m.queueCount.write(&b, "sandbox_queue_wait_seconds_count", "", "Requests that waited for memory.", "counter")
	m.proxyBytes.write(&b, "sandbox_proxy_bytes_total", "policy", "Bytes downloaded through the proxy.", "counter")
	m.proxyRefused.write(&b, "sandbox_proxy_refused_total", "policy", "Proxy connections refused or cut off.", "counter")
	m.browser.write(&b, "sandbox_browser_total", "action_outcome", "Browser requests by action and outcome.", "counter")
	m.sessions.write(&b, "sandbox_sessions_started_total", "image", "Sessions started, by image.", "counter")
	m.databases.write(&b, "sandbox_databases_total", "outcome", "Databases created, dropped, expired or cut off for size.", "counter")

	status := c.status()
	fmt.Fprintf(&b, "# HELP sandbox_paused 1 while the kill switch is on.\n# TYPE sandbox_paused gauge\nsandbox_paused %d\n", boolInt(status.Paused != ""))
	fmt.Fprintf(&b, "# HELP sandbox_memory_used_bytes Guest memory held by running and waiting microVMs.\n# TYPE sandbox_memory_used_bytes gauge\nsandbox_memory_used_bytes %d\n", status.MemoryUsed)
	fmt.Fprintf(&b, "# HELP sandbox_memory_cap_bytes Guest memory the microVMs may hold together.\n# TYPE sandbox_memory_cap_bytes gauge\nsandbox_memory_cap_bytes %d\n", status.MemoryCap)
	fmt.Fprintf(&b, "# HELP sandbox_queue_waiting Requests waiting for memory now.\n# TYPE sandbox_queue_waiting gauge\nsandbox_queue_waiting %d\n", status.Queued)
	fmt.Fprintf(&b, "# HELP sandbox_warm MicroVMs booted and waiting for a run.\n# TYPE sandbox_warm gauge\nsandbox_warm %d\n", status.Warm)
	fmt.Fprintf(&b, "# HELP sandbox_sessions Sessions running.\n# TYPE sandbox_sessions gauge\nsandbox_sessions %d\n", status.Sessions)
	fmt.Fprintf(&b, "# HELP sandbox_databases Databases alive.\n# TYPE sandbox_databases gauge\nsandbox_databases %d\n", status.Databases)
	fmt.Fprintf(&b, "# HELP sandbox_vms_running MicroVMs running, by image.\n# TYPE sandbox_vms_running gauge\n")
	for _, image := range sortedImages(status.Running) {
		fmt.Fprintf(&b, "sandbox_vms_running{image=%q} %d\n", image, status.Running[image])
	}
	return b.String()
}

func sortedImages(counts map[string]int) []string {
	images := make([]string, 0, len(counts))
	for image := range counts {
		images = append(images, image)
	}
	sort.Strings(images)
	return images
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
