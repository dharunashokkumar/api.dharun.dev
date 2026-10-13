#!/bin/sh
# Sets up the logos API on a Debian 12/13 machine or container. Run as root, with the
# other files of this directory next to it. Safe to re-run: it never overwrites the env file.
#
#   tar -C deploy -cz . | ssh <host> 'mkdir -p /root/logos-deploy && tar -xz -C /root/logos-deploy && sh /root/logos-deploy/install.sh'
#
# After this, new GitHub releases are installed automatically by logos-api-update.timer.
set -eu
here=$(cd "$(dirname "$0")" && pwd)
export DEBIAN_FRONTEND=noninteractive

[ "$(id -u)" = 0 ] || { echo "run as root" >&2; exit 1; }

# fonts-dejavu-core: the PNG renderer needs a font for the monogram fallback.
apt-get update -q
apt-get install -y -q curl ca-certificates gnupg fonts-dejavu-core

if ! command -v node >/dev/null 2>&1 || [ "$(node -p 'process.versions.node.split(".")[0]')" -lt 22 ]; then
  install -d -m 755 /etc/apt/keyrings
  curl -fsSL https://deb.nodesource.com/gpgkey/nodesource-repo.gpg.key | gpg --dearmor --yes -o /etc/apt/keyrings/nodesource.gpg
  echo "deb [signed-by=/etc/apt/keyrings/nodesource.gpg] https://deb.nodesource.com/node_24.x nodistro main" > /etc/apt/sources.list.d/nodesource.list
  apt-get update -q
  apt-get install -y -q nodejs
fi

id logos-api >/dev/null 2>&1 || adduser --system --group --home /opt/logos-api --no-create-home --shell /usr/sbin/nologin logos-api
install -d -m 755 /opt/logos-api /opt/logos-api/releases /opt/logos-api/failed /etc/logos-api

if [ ! -f /etc/logos-api/logos-api.env ]; then
  install -m 644 "$here/logos-api.env.example" /etc/logos-api/logos-api.env
  echo "Created /etc/logos-api/logos-api.env: set PUBLIC_ORIGIN, then: systemctl restart logos-api"
fi

install -m 755 "$here/logos-api-update.sh" /usr/local/bin/logos-api-update
install -m 644 "$here/logos-api.service" "$here/logos-api-update.service" "$here/logos-api-update.timer" /etc/systemd/system/
systemctl daemon-reload
systemctl enable logos-api.service logos-api-update.timer

# First release: fetched by the same updater the timer runs. It starts the service itself.
systemctl start logos-api-update.service || { journalctl -u logos-api-update.service --no-pager -n 20; exit 1; }
systemctl start logos-api-update.timer

echo "INSTALL OK: logos-api $(cat /opt/logos-api/current/VERSION), $(systemctl is-active logos-api.service)"
