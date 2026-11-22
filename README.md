# mesh

A self-hosted mesh networking system. Phase 1 is a Linux-first WireGuard hub-and-spoke MVP with a central Go control server. The control plane handles enrollment and peer configuration; WireGuard carries encrypted traffic.

> Phase 1 status: source scaffold / development MVP. Do not expose the control API directly to the public internet. Put it behind HTTPS (Caddy/Nginx), use a long random enrollment token, and restrict the admin token. The first release supports Linux clients only. Windows/macOS/Android/iOS require platform-specific service/VPN integrations.

## Architecture

- Control server: Go `net/http`, JSON file-backed state (single-node MVP), WireGuard peer provisioning via `wg set`.
- Agent: Go CLI; generates a WireGuard key locally, enrolls over HTTPS, writes a root-only `wg-quick` config, and starts the tunnel.
- Data plane: hub-and-spoke WireGuard. All nodes route the mesh prefix through the VPS. Enable IPv4 forwarding on the VPS.
- Later: replace hub routing with peer endpoint discovery + NAT traversal + encrypted relay, add database and dashboard.

## Requirements

Server: Ubuntu 24.04+, Go 1.23+ to build, `wireguard-tools`, public UDP 51820, TCP 443 via reverse proxy.

Client (Phase 1): Linux, root/sudo, Go 1.23+ to build, `wireguard-tools`, `iproute2`, `wg-quick`.

## Build

```bash
go build -o bin/mesh-server ./cmd/mesh-server
go build -o bin/mesh-agent ./cmd/mesh-agent
```

## Server setup (Ubuntu)

1. Install tools:

```bash
sudo apt update
sudo apt install -y wireguard-tools iproute2 curl
```

2. Create server keys and interface:

```bash
sudo install -d -m 700 /etc/mesh
umask 077
wg genkey | sudo tee /etc/mesh/server.key | wg pubkey | sudo tee /etc/mesh/server.pub
```

3. Create `/etc/mesh/server.json` from `configs/server.example.json`. Set the actual public DNS name, server public key, enrollment token, admin token, and server private key path. Keep the private key secret.

4. Configure `/etc/wireguard/wg-mesh.conf` (see `configs/wg-mesh-server.conf.example`) and bring it up:

```bash
sudo install -m 600 configs/wg-mesh-server.conf.example /etc/wireguard/wg-mesh.conf
# Edit the PrivateKey line to contain the contents of /etc/mesh/server.key
sudo systemctl enable --now wg-quick@wg-mesh
sudo sysctl -w net.ipv4.ip_forward=1
```

Persist forwarding with `/etc/sysctl.d/99-mesh.conf` containing `net.ipv4.ip_forward=1`.

5. Build/install server:

```bash
sudo install -m 755 bin/mesh-server /usr/local/bin/mesh-server
sudo install -m 600 configs/server.example.json /etc/mesh/server.json
# Edit /etc/mesh/server.json before starting.
sudo install -m 644 deploy/systemd/mesh-server.service /etc/systemd/system/mesh-server.service
sudo systemctl daemon-reload
sudo systemctl enable --now mesh-server
```

The server listens on `127.0.0.1:8080` by default. Use Caddy to terminate HTTPS for `mesh.sakshamchugh.com` and proxy to `127.0.0.1:8080`. WireGuard UDP 51820 must be forwarded directly to the VPS.

Example Caddyfile:

```caddy
mesh.sakshamchugh.com {
    reverse_proxy 127.0.0.1:8080
}
```

Open inbound TCP 80/443 and UDP 51820 in the VPS firewall/security group. Do not expose TCP 8080.

## DNS and firewall

Create an A/AAAA record for `mesh.sakshamchugh.com` pointing to the VPS. Allow inbound TCP 80/443 and UDP 51820. Also permit forwarding between peers on the VPS; the exact firewall commands depend on whether the VPS uses UFW, nftables, or provider firewall rules. Do not expose the control API on TCP 8080.

## Enroll a Linux client

Build/install `mesh-agent`, then run:

```bash
sudo mesh-agent join \
  --server https://mesh.sakshamchugh.com \
  --token 'YOUR_ENROLLMENT_TOKEN' \
  --name 'homeserver'
```

The agent creates `/etc/mesh/agent.json`, `/etc/wireguard/wg-mesh.conf`, and a `mesh-agent` systemd heartbeat service, then starts both services. Enable the tunnel at boot:

```bash
sudo systemctl enable wg-quick@wg-mesh mesh-agent
```

Check:

```bash
sudo wg show wg-mesh
ip addr show wg-mesh
ping 100.80.0.1
```

Enroll a second client the same way. Nodes should be able to ping each other via the hub if host firewalls permit ICMP and forwarding is enabled.

## API

- `GET /healthz` public health check
- `POST /v1/devices/register` enrollment token required
- `POST /v1/devices/heartbeat` per-device bearer token required
- `GET /v1/devices` admin bearer token required
- `DELETE /v1/devices/{id}` admin bearer token required

## Important limitations in this MVP

- Hub-and-spoke only; no direct peer-to-peer NAT traversal or relay yet.
- The hub is in the data path, so it is a bandwidth bottleneck and a single point of failure.
- A central control server going offline does not terminate already established WireGuard tunnels, but new enrollment/config updates and liveness reporting stop.
- The JSON state store is intended for an early single-process MVP, not high availability or multi-instance deployment.
- The install script expects published release binaries; it does not compile code fetched from the internet.
- This repository is a starting implementation, not a security-audited VPN product.
