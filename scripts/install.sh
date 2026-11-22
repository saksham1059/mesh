#!/usr/bin/env sh
set -eu
REPO="${MESH_REPO:-saksham1059/mesh}"
VERSION="${MESH_VERSION:-latest}"
ARCH="$(uname -m)"
case "$ARCH" in x86_64) ARCH=amd64;; aarch64|arm64) ARCH=arm64;; *) echo "Unsupported architecture: $ARCH" >&2; exit 1;; esac
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ASSET="mesh-agent-${OS}-${ARCH}"
if [ "$VERSION" = latest ]; then URL="https://github.com/${REPO}/releases/latest/download/${ASSET}"; else URL="https://github.com/${REPO}/releases/download/${VERSION}/${ASSET}"; fi
TMP="$(mktemp)"
trap 'rm -f "$TMP"' EXIT
curl -fsSL "$URL" -o "$TMP"
chmod 0755 "$TMP"
sudo install -m 0755 "$TMP" /usr/local/bin/mesh-agent
sudo install -d -m 0700 /etc/mesh /etc/wireguard
if [ ! -f /etc/systemd/system/mesh-agent.service ]; then
  echo "Installed mesh-agent. Enroll with: sudo mesh-agent join --server https://mesh.sakshamchugh.com --token TOKEN --name DEVICE"
fi
