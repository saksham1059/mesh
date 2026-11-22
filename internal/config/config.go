package config

import (
	"encoding/json"
	"fmt"
	"os"
)

type Server struct {
	ListenAddr             string `json:"listen_addr"`
	PublicEndpoint         string `json:"public_endpoint"`
	WGInterface            string `json:"wireguard_interface"`
	WGServerPublicKey      string `json:"wireguard_server_public_key"`
	WGServerPrivateKeyFile string `json:"wireguard_server_private_key_file"`
	EnrollmentToken        string `json:"enrollment_token"`
	AdminToken             string `json:"admin_token"`
	StateFile              string `json:"state_file"`
	MeshCIDR               string `json:"mesh_cidr"`
	ServerAddress          string `json:"server_address"`
}

func LoadServer(path string) (Server, error) {
	var c Server
	b, err := os.ReadFile(path)
	if err != nil {
		return c, err
	}
	if err = json.Unmarshal(b, &c); err != nil {
		return c, err
	}
	if c.ListenAddr == "" || c.PublicEndpoint == "" || c.WGInterface == "" || c.WGServerPublicKey == "" || c.EnrollmentToken == "" || c.AdminToken == "" || c.StateFile == "" || c.MeshCIDR == "" {
		return c, fmt.Errorf("missing required server configuration")
	}
	if c.WGServerPrivateKeyFile == "" {
		return c, fmt.Errorf("wireguard_server_private_key_file is required")
	}
	return c, nil
}
