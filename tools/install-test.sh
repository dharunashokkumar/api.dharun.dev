#!/usr/bin/env bash
# Installs a built bundle the way a real server gets it, inside a Debian 13 container
# running systemd, and checks that everything a fresh machine needs really happens:
#
#   deploy/install.sh creates the user, units and timers
#   the updater downloads the release, verifies it and starts the service
#   the service runs sandboxed as its own user and answers with the right version
#   the refresh job can fetch a dataset and the running API sees the result
#   running the installer again changes nothing
#
#   bash tools/install-test.sh dist      # the directory build-release.sh wrote
#
# Needs Docker. The release is served to the container from inside it, so nothing is
# downloaded from GitHub; the refresh step fetches one small public list.
#
# Commands for the container are written in single quotes on purpose: they are expanded
# by the shell inside it, not by this one.
# shellcheck disable=SC2016
set -euo pipefail
cd "$(dirname "$0")/.."

dist="$(cd "${1:?usage: install-test.sh <dist directory>}" && pwd)"
version="$(cat "$dist/VERSION")"
image=dharun-api-install-test
name="dharun-api-install-test-$$"

docker build -q -t "$image" - >/dev/null <<'EOF'
FROM debian:13
RUN apt-get update -q && apt-get install -y -q --no-install-recommends systemd systemd-sysv curl ca-certificates python3 \
    && rm -rf /var/lib/apt/lists/*
STOPSIGNAL SIGRTMIN+3
CMD ["/sbin/init"]
EOF

docker run -d --name "$name" --privileged --cgroupns=host --tmpfs /run --tmpfs /run/lock --tmpfs /tmp \
  -v /sys/fs/cgroup:/sys/fs/cgroup:rw -v "$dist:/release:ro" -v "$PWD/deploy:/deploy:ro" "$image" >/dev/null
trap 'docker rm -f "$name" >/dev/null 2>&1 || true' EXIT

run() { docker exec "$name" bash -euo pipefail -c "$1"; }
fail() {
  echo "FAIL: $*" >&2
  run 'systemctl --no-pager status dharun-api.service dharun-api-update.service dharun-api-refresh.service 2>&1 | tail -40; journalctl --no-pager -n 40 2>&1' >&2 || true
  exit 1
}

for _ in $(seq 1 60); do
  state="$(docker exec "$name" systemctl is-system-running 2>/dev/null || true)"
  case "$state" in running|degraded) break ;; esac
  sleep 1
done
case "$state" in running|degraded) ;; *) fail "systemd did not come up in the container (state: $state)" ;; esac

# The release server: the bundle exactly as the publish job uploads it.
run 'systemd-run --quiet --unit=release-server python3 -m http.server 9000 --bind 127.0.0.1 --directory /release'
run 'install -d /etc/dharun-api && cp /deploy/dharun-api.env.example /etc/dharun-api/env && echo API_RELEASE_URL=http://127.0.0.1:9000 >> /etc/dharun-api/env'
run 'for _ in $(seq 1 30); do curl -fsS -o /dev/null http://127.0.0.1:9000/VERSION && exit 0; sleep 0.5; done; exit 1' || fail "the release server did not start"

run 'mkdir /root/api-deploy && cp -r /deploy/. /root/api-deploy/ && sh /root/api-deploy/install.sh' | tail -3 || fail "install.sh failed"
echo "ok   install.sh"

health="$(run 'curl -fsS http://127.0.0.1:8080/healthz')" || fail "the service does not answer"
case "$health" in
  "{\"status\":\"ok\",\"version\":\"$version\","*) echo "ok   service answers as $version" ;;
  *) fail "/healthz is not version $version: ${health:0:120}" ;;
esac

[ "$(run 'systemctl show -p User --value dharun-api.service')" = dharun-api ] || fail "the service does not run as dharun-api"
[ "$(run 'stat -c %U "/proc/$(systemctl show -p MainPID --value dharun-api.service)"')" = dharun-api ] || fail "the server process is not owned by dharun-api"
echo "ok   runs as dharun-api"

for timer in dharun-api-update.timer dharun-api-refresh.timer; do
  [ "$(run "systemctl is-active $timer")" = active ] || fail "$timer is not active"
  [ "$(run "systemctl is-enabled $timer")" = enabled ] || fail "$timer is not enabled"
done
echo "ok   timers"

# The sandbox holds: the release directory is read-only to the service.
if run 'systemd-run --quiet --wait --pipe -p User=dharun-api -p ProtectSystem=strict touch /opt/dharun-api/current/x' 2>/dev/null; then
  fail "the service user can write into the release"
fi
echo "ok   release is read-only to the service"

# An image is rendered: resvg runs inside the sandbox.
run 'curl -fsS -o /tmp/og.png "http://127.0.0.1:8080/v1/og-image?title=Install%20test" && head -c 8 /tmp/og.png | grep -q PNG' || fail "PNG rendering does not work in the service"
echo "ok   renders PNG"

# The refresh job, with the unit's own user, sandbox and limits, limited here to one
# small dataset so the test does not download the large ones.
run 'mkdir -p /etc/systemd/system/dharun-api-refresh.service.d && printf "[Service]\nExecStart=\nExecStart=/opt/dharun-api/current/apidata refresh -force disposable-email\n" > /etc/systemd/system/dharun-api-refresh.service.d/test.conf && systemctl daemon-reload'
run 'systemctl start dharun-api-refresh.service' || fail "the refresh job failed"
run 'test -s /var/lib/dharun-api/data/refresh.json' || fail "the refresh job left no record in /var/lib/dharun-api/data"
# The source may or may not have changed since the seed was built; either way the job
# reached it, and the running API reports the outcome without a restart.
seen=no
for _ in $(seq 1 40); do
  if run 'curl -fsS http://127.0.0.1:8080/healthz' |
    grep -Eq '"disposable-email":\{"copy":"(seed|server)","fetched":"[^"]*","records":\{[^}]*\},"refresh":"(updated|unchanged)"'; then
    seen=yes
    break
  fi
  sleep 1
done
[ "$seen" = yes ] || fail "the API does not report the refresh: $(run 'curl -fsS http://127.0.0.1:8080/healthz' | head -c 600)"
echo "ok   refresh job reaches its source, and the API sees the result without a restart"
run 'rm -r /etc/systemd/system/dharun-api-refresh.service.d && systemctl daemon-reload'

# Nothing to do the second time: same version, env file untouched, still running.
before="$(run 'sha256sum /etc/dharun-api/env; systemctl show -p MainPID --value dharun-api.service')"
run 'sh /root/api-deploy/install.sh' | tail -1 || fail "running install.sh again failed"
after="$(run 'sha256sum /etc/dharun-api/env; systemctl show -p MainPID --value dharun-api.service')"
[ "$before" = "$after" ] || fail "running install.sh again changed the env file or restarted the service"
echo "ok   install.sh is safe to run again"

echo "install test passed: $version"
