package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type registration struct {
	ID                  string `json:"id"`
	Name                string `json:"name"`
	IP                  string `json:"ip"`
	DeviceToken         string `json:"device_token"`
	ServerPublicKey     string `json:"server_public_key"`
	Endpoint            string `json:"endpoint"`
	AllowedIPs          string `json:"allowed_ips"`
	PersistentKeepalive int    `json:"persistent_keepalive"`
}
type localConfig struct {
	Server string `json:"server"`
	ID     string `json:"id"`
	Name   string `json:"name"`
	IP     string `json:"ip"`
	Token  string `json:"token"`
}

func mustRoot() {
	if os.Geteuid() != 0 {
		fmt.Fprintln(os.Stderr, "run this command with sudo/root")
		os.Exit(1)
	}
}
func wgKey() (string, string, error) {
	priv, e := exec.Command("wg", "genkey").Output()
	if e != nil {
		return "", "", e
	}
	pubCmd := exec.Command("wg", "pubkey")
	pubCmd.Stdin = bytes.NewReader(priv)
	pub, e := pubCmd.Output()
	if e != nil {
		return "", "", e
	}
	return strings.TrimSpace(string(priv)), strings.TrimSpace(string(pub)), nil
}
func write(path string, b []byte, mode os.FileMode) error {
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return e
	}
	return os.WriteFile(path, b, mode)
}
func join(args []string) {
	fs := flag.NewFlagSet("join", flag.ExitOnError)
	server := fs.String("server", "", "Control server URL")
	token := fs.String("token", "", "Enrollment token")
	name := fs.String("name", "", "Device name")
	_ = fs.Parse(args)
	mustRoot()
	if *server == "" || *token == "" || *name == "" {
		fmt.Fprintln(os.Stderr, "usage: mesh-agent join --server https://host --token TOKEN --name DEVICE")
		os.Exit(2)
	}
	priv, pub, e := wgKey()
	if e != nil {
		fatal("generate WireGuard keys", e)
	}
	payload, _ := json.Marshal(map[string]string{"name": *name, "public_key": pub, "enrollment_token": *token})
	req, e := http.NewRequest("POST", strings.TrimRight(*server, "/")+"/v1/devices/register", bytes.NewReader(payload))
	if e != nil {
		fatal("create request", e)
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 15 * time.Second}
	resp, e := client.Do(req)
	if e != nil {
		fatal("register device", e)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != 201 {
		fatal("registration rejected", fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body))))
	}
	var r registration
	if e = json.Unmarshal(body, &r); e != nil {
		fatal("parse registration response", e)
	}
	wgConf := fmt.Sprintf("[Interface]\nPrivateKey = %s\nAddress = %s/16\n\n[Peer]\nPublicKey = %s\nEndpoint = %s\nAllowedIPs = %s\nPersistentKeepalive = %d\n", priv, r.IP, r.ServerPublicKey, r.Endpoint, r.AllowedIPs, r.PersistentKeepalive)
	if e = write("/etc/wireguard/wg-mesh.conf", []byte(wgConf), 0600); e != nil {
		fatal("write WireGuard config", e)
	}
	lc := localConfig{Server: strings.TrimRight(*server, "/"), ID: r.ID, Name: r.Name, IP: r.IP, Token: r.DeviceToken}
	b, _ := json.MarshalIndent(lc, "", "  ")
	if e = write("/etc/mesh/agent.json", b, 0600); e != nil {
		fatal("write agent config", e)
	}
	fmt.Printf("Registered %s as %s (%s)\n", r.Name, r.ID, r.IP)
	c := exec.Command("systemctl", "enable", "--now", "wg-quick@wg-mesh")
	out, e := c.CombinedOutput()
	if e != nil {
		fmt.Fprintf(os.Stderr, "WireGuard config saved, but tunnel startup failed: %v: %s\n", e, out)
		os.Exit(1)
	}
	unit := `[Unit]
Description=mesh node agent heartbeat
After=network-online.target wg-quick@wg-mesh.service
Wants=network-online.target wg-quick@wg-mesh.service

[Service]
Type=simple
User=root
ExecStart=/usr/local/bin/mesh-agent run
Restart=always
RestartSec=5
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true

[Install]
WantedBy=multi-user.target
`
	if e = write("/etc/systemd/system/mesh-agent.service", []byte(unit), 0644); e != nil {
		fatal("install agent service", e)
	}
	_ = exec.Command("systemctl", "daemon-reload").Run()
	if out, e = exec.Command("systemctl", "enable", "--now", "mesh-agent").CombinedOutput(); e != nil {
		fmt.Fprintf(os.Stderr, "warning: heartbeat service could not start: %v: %s\n", e, out)
	}
	fmt.Println("mesh tunnel is up. Check with: sudo mesh-agent status")
}
func status() {
	mustRoot()
	b, e := os.ReadFile("/etc/mesh/agent.json")
	if e != nil {
		fatal("read agent config", e)
	}
	var c localConfig
	_ = json.Unmarshal(b, &c)
	fmt.Printf("Device: %s\nID: %s\nMesh IP: %s\n", c.Name, c.ID, c.IP)
	out, e := exec.Command("wg", "show", "wg-mesh").CombinedOutput()
	if e != nil {
		fmt.Printf("Tunnel: down (%v)\n", e)
	} else {
		fmt.Printf("Tunnel: up\n%s", out)
	}
}
func runDaemon() {
	mustRoot()
	b, e := os.ReadFile("/etc/mesh/agent.json")
	if e != nil {
		fatal("read agent config", e)
	}
	var c localConfig
	if e = json.Unmarshal(b, &c); e != nil {
		fatal("parse agent config", e)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	for {
		req, err := http.NewRequest("POST", c.Server+"/v1/devices/heartbeat", bytes.NewReader([]byte(`{}`)))
		if err == nil {
			req.Header.Set("Authorization", "Bearer "+c.Token)
			req.Header.Set("Content-Type", "application/json")
			resp, er := client.Do(req)
			if er == nil {
				_, _ = io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
			}
		}
		time.Sleep(30 * time.Second)
	}
}
func fatal(s string, e error) { fmt.Fprintf(os.Stderr, "mesh-agent: %s: %v\n", s, e); os.Exit(1) }
func main() {
	if len(os.Args) < 2 {
		fmt.Println("mesh-agent <join|status|run>")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "join":
		join(os.Args[2:])
	case "status":
		status()
	case "run":
		runDaemon()
	default:
		fmt.Println("unknown command")
		os.Exit(2)
	}
}
