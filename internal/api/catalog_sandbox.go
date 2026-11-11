package api

// Running code (sandbox.go).

func withSandbox(groups []apiGroup) []apiGroup {
	return append(groups, apiGroup{
		ID: "run", Category: "developer", Title: "Run code",
		Tagline: "Run a Python or JavaScript program once in a throwaway microVM and get its output.",
		Intro:   "Send a program and get back what it printed, its exit code and the time it took. Each run gets a fresh Firecracker microVM with no network, 30 seconds, 256 MB of memory and 64 KB of output, thrown away after. Needs a free API key from the start: 50 runs and 10 CPU minutes a day per key. What ran is recorded as a hash for 30 days, never the code.",
		Endpoints: []endpoint{
			{Path: "/v1/run", Summary: "Run a program", Private: true, Key: true,
				Doc:         "JSON {\"language\": \"python\" or \"javascript\", \"code\": \"...\", \"stdin\": \"...\"}. Returns stdout, stderr, exitCode, durationMs, cpuMs and limitsHit (time, memory, output, processes). 401 without a key; 503 sandbox_off while the sandbox is switched off.",
				Example:     "/v1/run",
				Upload:      &upload{Accepts: `JSON {"language": "python", "code": "print(sum(range(10)))"}, up to 64 KB of code.`, Types: []string{"application/json"}, JSON: true},
				ExampleBody: `{"language": "python", "code": "import sys\nprint('Vanakkam from', sys.version.split()[0])"}`},
			{Path: "/v1/run/languages", Summary: "The languages you can run", Example: "/v1/run/languages"},
		},
	})
}
