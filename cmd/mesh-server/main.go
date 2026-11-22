package main

import (
	"flag"
	"github.com/saksham1059/mesh/internal/api"
	"github.com/saksham1059/mesh/internal/config"
	"github.com/saksham1059/mesh/internal/store"
	"github.com/saksham1059/mesh/internal/wireguard"
	"log"
	"net/http"
	"os"
)

func main() {
	cfg := flag.String("config", "/etc/mesh/server.json", "server config path")
	flag.Parse()
	c, e := config.LoadServer(*cfg)
	if e != nil {
		log.Fatal(e)
	}
	if _, e = os.Stat(c.WGServerPrivateKeyFile); e != nil {
		log.Fatalf("WireGuard server private key file: %v", e)
	}
	st, e := store.New(c.StateFile, c.MeshCIDR)
	if e != nil {
		log.Fatal(e)
	}
	// Reconcile persisted devices into the live WireGuard interface after server restarts.
	for _, d := range st.List() {
		if err := wireguard.AddPeer(c.WGInterface, d.PublicKey, d.IP); err != nil {
			log.Printf("warning: restore peer %s: %v", d.ID, err)
		}
	}
	srv := &api.Server{C: c, S: st}
	httpSrv := &http.Server{Addr: c.ListenAddr, Handler: srv.Handler(), ReadHeaderTimeout: 5_000_000_000}
	log.Printf("mesh control listening on %s", c.ListenAddr)
	log.Fatal(httpSrv.ListenAndServe())
}
