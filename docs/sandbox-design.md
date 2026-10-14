# Code sandbox: design for a decision

Status: **Open**. Nothing here is built. Written 2026-10-14 from the brainstorm recorded
in `ROADMAP.md` (section 6, "Disposable services and sandboxes"), for Dharun to decide
on. Network addresses, VM names and host details belong in the private homelab
repository, not here.

## What it is

`POST /v1/sandbox/run`: code in, output out. Python or Node.js, run once in a throwaway
virtual machine that is destroyed afterwards. Aimed at AI agent developers who need a
safe place to run generated code (the E2B use case), and at learners.

```json
POST /v1/sandbox/run
{ "language": "python", "code": "print(sum(range(10)))", "stdin": "" }

{ "data": { "stdout": "45\n", "stderr": "", "exitCode": 0, "durationMs": 212,
            "limitsHit": [] }, "meta": { ... } }
```

Phase 1 is only this. Phase 2 (a sandbox kept for five minutes, files, `pip` and `npm`
through a proxy) and phase 3 (SSH for learners) come later and reuse everything below.

## Why it reopens settled decisions

| Settled decision | What the sandbox needs instead |
|---|---|
| One container | A separate VM with nested virtualization, on its own isolated network |
| No API keys, rate limit per IP | A login and a daily quota per user, for the sandbox endpoints only |
| Memory up to 512 MB or 1 GB | About 3 GB for the sandbox VM |
| Go | Unchanged if the controller is written in Go (`firecracker-go-sdk`, Apache-2.0) |

The rest of the API stays as it is: no keys, per-IP limits, one container.

## Layout

```text
caller ── Cloudflare ── tunnel ── API container ──(private link)── sandbox VM
                                   checks login,                    controller (Go)
                                   quota, Turnstile                 jailer + Firecracker
                                                                    microVM per run
```

- **The API** stays the only public entry. It checks the caller's login and quota, then
  forwards the request to the controller over a private link that nothing else can reach.
- **The sandbox VM** is a dedicated Proxmox VM with nested virtualization, on an isolated
  network with no route to the LAN or the Proxmox interface. Outgoing traffic is dropped
  except the private link (and, in phase 2, the package proxy). An escape from a microVM
  lands in this VM, which holds nothing worth taking and can be rebuilt from a template.
- **The controller** is a small Go service in that VM. It keeps a pool of microVMs restored
  from a snapshot, hands one to each run, collects the output, and destroys it.

Firecracker on the Proxmox host itself is ruled out (an escape reaches the whole
homelab), and inside an LXC container the host kernel is shared, which defeats the point.

## One run

1. The API checks the login (see below), the quota and the size of the code (64 KB),
   then calls the controller.
2. The controller takes a microVM that was restored from the snapshot of an already
   booted guest (Python and Node.js loaded), so a run does not wait for a boot.
3. The code goes in over vsock to a small agent in the guest (a Go binary as PID 1), which
   runs it as an unprivileged user with the time limit, and streams stdout and stderr back,
   each cut at 64 KB.
4. At exit, at the time limit, or when the output limit is hit, the controller kills the
   microVM and deletes its writable disk. Nothing from one run reaches the next.

Each microVM:

| Limit | Value | Enforced by |
|---|---|---|
| Time | 30 s wall clock | controller kills the VM |
| CPU | 1 vCPU, capped at one core | jailer cgroup (`cpu.max`) |
| Memory | 256 MB | microVM size; the cgroup as a backstop |
| Disk | read-only root, 256 MB scratch | a fresh overlay file per run |
| Disk I/O | rate-limited | Firecracker's drive rate limiter |
| Network | none: no network device at all | microVM configuration |
| Processes | 64 | guest agent (`RLIMIT_NPROC`) |

With a 3 GB VM that is 8 to 10 runs at once; more wait in a queue, like the heavy routes
do now, and are refused after 10 seconds.

## Login and quota

A Turnstile check needs a browser, and agents and command-line tools have none. So:

- **Login with GitHub** (OAuth device flow, which works from a terminal) gives a sandbox
  token. Tokens are only for `/v1/sandbox`; the rest of the API never asks for one.
- **A daily quota per GitHub account**, for example 200 runs and 30 CPU-minutes, and
  a lower one for accounts younger than 30 days.
- **Turnstile** in front of any browser playground, verified with the widget's own
  secret, not the API token.
- Per-IP limits stay as a second line, but blocking falls on the account, never on an
  address, so apps behind Vercel or Cloudflare Workers are not caught (`ROADMAP.md`,
  Cloudflare automation).

## Monitoring and abuse

From `ROADMAP.md`, "Monitoring for these services":

- Metrics per run: CPU time, peak memory, duration, exit reason (exit, time limit, memory,
  output limit). Pool size, queue length and wait time.
- Alerts: runs that keep the CPU at its cap until the time limit (mining), runs that try
  to reach the network, the same account hitting limits again and again.
- An audit log in Loki, never public: account, run ID, a SHA-256 of the code, limits hit.
  Not the code itself. Retention to be decided.
- A kill switch: one command (or a file the controller watches) refuses new runs and
  stops the running ones; the dashboard shows whether it is on and why.

## Alternatives

| Option | Isolation | Needs | Verdict |
|---|---|---|---|
| Firecracker microVMs in a dedicated VM | Separate kernel per run, inside a throwaway VM | Nested virtualization | **Recommended**; what E2B and AWS Lambda use |
| gVisor (`runsc`) | User-space kernel intercepting system calls | No KVM; can run in a container | Simpler, weaker; a fallback if nested virtualization is not possible |
| Containers with seccomp (nsjail, bubblewrap) | Shared kernel | Nothing | Not enough for strangers' code |
| A paid service (E2B) | Theirs | Money, a key on our side | Against "free, no keys" |

E2B's own infrastructure (`e2b-dev/infra`, Apache-2.0) is a Firecracker orchestrator in Go
and is worth reading, and borrowing from, before writing the controller.

## Decisions needed

1. Build it at all, accepting the four reopened decisions above.
2. Firecracker in a dedicated VM, or gVisor to start with.
3. GitHub login for sandbox tokens, or another way to identify callers.
4. The quotas: runs and CPU-minutes per day, and for new accounts.
5. How long the audit log is kept.
6. How much of the host the sandbox VM gets: 3 GB and two cores would be a start.
7. Languages for phase 1: Python and Node.js, or more.

Measure before trusting the numbers here: snapshot restore time, memory per microVM with
shared snapshot pages, and how many runs the VM holds at once.
