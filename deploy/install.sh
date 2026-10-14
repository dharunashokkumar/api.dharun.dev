#!/bin/sh
# Sets up the API on a Debian 12/13 machine or container. Run as root, with the other
# files of this directory next to it. Safe to re-run: it never overwrites the env file.
#
#   tar -C deploy -cz . | ssh <host> 'mkdir -p /root/api-deploy && tar -xz -C /root/api-deploy && sh /root/api-deploy/install.sh'
#
# After this, new GitHub releases are installed automatically by dharun-api-update.timer,
# and datasets are refreshed from their sources by dharun-api-refresh.timer.
# The server is one static binary. The file tools (OCR, images, PDF text, audio) also run
# programs from Debian, installed here; the rest come in the release bundle.
set -eu
here=$(cd "$(dirname "$0")" && pwd)
export DEBIAN_FRONTEND=noninteractive

[ "$(id -u)" = 0 ] || { echo "run as root" >&2; exit 1; }

apt-get update -q
apt-get install -y -q curl ca-certificates
# Tesseract with English and ten Indian languages; libvips (with AVIF output); ffmpeg;
# poppler's pdftotext and pdftoppm. About 600 MB. prlimit (util-linux) caps Typst's memory.
apt-get install -y -q --no-install-recommends   tesseract-ocr tesseract-ocr-eng tesseract-ocr-hin tesseract-ocr-tam tesseract-ocr-tel tesseract-ocr-kan   tesseract-ocr-mal tesseract-ocr-ben tesseract-ocr-guj tesseract-ocr-pan tesseract-ocr-ori tesseract-ocr-mar   libvips-tools libheif-plugin-aomenc ffmpeg poppler-utils util-linux

# useradd, not adduser: a minimal Debian 13 does not come with adduser.
id dharun-api >/dev/null 2>&1 || useradd --system --user-group --home-dir /opt/dharun-api --no-create-home --shell /usr/sbin/nologin dharun-api
install -d -m 755 /opt/dharun-api /opt/dharun-api/releases /opt/dharun-api/failed /etc/dharun-api

# The traceroute opens ICMP datagram sockets, which the kernel allows only to the groups
# in net.ipv4.ping_group_range. systemd opens it to every group at boot, but inside an
# unprivileged container (Proxmox LXC) that setting fails and the range stays empty; then
# it is opened to the service's group alone, now and at every boot.
# (Read with cat: sh's read takes a byte at a time, and the kernel ends a sysctl file
# after the first.)
gid=$(id -g dharun-api)
range=$(cat /proc/sys/net/ipv4/ping_group_range)
low=${range%%[[:space:]]*}
high=${range##*[[:space:]]}
if [ "$gid" -lt "$low" ] || [ "$gid" -gt "$high" ]; then
  install -d -m 755 /etc/sysctl.d
  echo "net.ipv4.ping_group_range = $gid $gid" > /etc/sysctl.d/60-dharun-api.conf
  echo "$gid $gid" > /proc/sys/net/ipv4/ping_group_range ||
    echo "warning: could not set net.ipv4.ping_group_range; /v1/mtr will answer 503" >&2
fi

if [ ! -f /etc/dharun-api/env ]; then
  install -m 644 "$here/dharun-api.env.example" /etc/dharun-api/env
  echo "Created /etc/dharun-api/env: check PUBLIC_ORIGIN, then: systemctl restart dharun-api"
fi

install -m 755 "$here/dharun-api-update.sh" /usr/local/bin/dharun-api-update
install -m 644 "$here/dharun-api.service" "$here/dharun-api-update.service" "$here/dharun-api-update.timer"   "$here/dharun-api-refresh.service" "$here/dharun-api-refresh.timer" /etc/systemd/system/
systemctl daemon-reload
systemctl enable dharun-api.service dharun-api-update.timer dharun-api-refresh.timer

# First release: fetched by the same updater the timer runs. It starts the service itself.
systemctl start dharun-api-update.service || { journalctl -u dharun-api-update.service --no-pager -n 20; exit 1; }
systemctl start dharun-api-update.timer dharun-api-refresh.timer

echo "INSTALL OK: dharun-api $(cat /opt/dharun-api/current/VERSION), $(systemctl is-active dharun-api.service)"
