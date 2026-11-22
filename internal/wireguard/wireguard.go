package wireguard

import (
	"fmt"
	"os/exec"
	"strings"
)

func run(args ...string) error {
	c := exec.Command("wg", args...)
	out, e := c.CombinedOutput()
	if e != nil {
		return fmt.Errorf("wg %s: %w: %s", strings.Join(args, " "), e, strings.TrimSpace(string(out)))
	}
	return nil
}
func AddPeer(iface, pub, ip string) error {
	return run("set", iface, "peer", pub, "allowed-ips", ip+"/32")
}
func RemovePeer(iface, pub string) error { return run("set", iface, "peer", pub, "remove") }
