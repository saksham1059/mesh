#!/usr/bin/env bash
set -euo pipefail
if [[ $EUID -ne 0 ]]; then echo "Run with sudo" >&2; exit 1; fi
apt-get update
apt-get install -y wireguard-tools iproute2 curl ca-certificates
install -d -m 0700 /etc/mesh /var/lib/mesh
install -d -m 0700 /etc/wireguard
if [[ ! -s /etc/mesh/server.key ]]; then
  umask 077
  wg genkey > /etc/mesh/server.key
  wg pubkey < /etc/mesh/server.key > /etc/mesh/server.pub
fi
PUB=$(cat /etc/mesh/server.pub)
ENROLL=$(openssl rand -hex 32)
ADMIN=$(openssl rand -hex 32)
cat > /etc/mesh/server.json <<JSON
{
  "listen_addr":"127.0.0.1:8080",
  "public_endpoint":"mesh.sakshamchugh.com:51820",
  "wireguard_interface":"wg-mesh",
  "wireguard_server_public_key":"$PUB",
  "wireguard_server_private_key_file":"/etc/mesh/server.key",
  "enrollment_token":"$ENROLL",
  "admin_token":"$ADMIN",
  "state_file":"/var/lib/mesh/state.json",
  "mesh_cidr":"100.80.0.0/16",
  "server_address":"100.80.0.1/16"
}
JSON
chmod 600 /etc/mesh/server.json
cat > /etc/wireguard/wg-mesh.conf <<WG
[Interface]
Address = 100.80.0.1/16
ListenPort = 51820
PrivateKey = $(cat /etc/mesh/server.key)
SaveConfig = false
WG
chmod 600 /etc/wireguard/wg-mesh.conf
cat >/etc/sysctl.d/99-mesh.conf <<SYS
net.ipv4.ip_forward=1
SYS
sysctl --system >/dev/null
systemctl enable --now wg-quick@wg-mesh
if [[ -x ./bin/mesh-server ]]; then
  install -m 0755 ./bin/mesh-server /usr/local/bin/mesh-server
  install -m 0644 ./deploy/systemd/mesh-server.service /etc/systemd/system/mesh-server.service
  systemctl daemon-reload
  systemctl enable --now mesh-server
else
  echo "No ./bin/mesh-server found; build it with 'make build', install it to /usr/local/bin/mesh-server, then install deploy/systemd/mesh-server.service."
fi
printf 'Server keys and config created. Enrollment token: %s\nAdmin token: %s\nSave both securely.\n' "$ENROLL" "$ADMIN"
