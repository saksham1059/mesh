package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/saksham1059/mesh/internal/config"
	"github.com/saksham1059/mesh/internal/store"
	"github.com/saksham1059/mesh/internal/wireguard"
)

type Server struct {
	C config.Server
	S *store.Store
}
type registerRequest struct {
	Name            string `json:"name"`
	PublicKey       string `json:"public_key"`
	EnrollmentToken string `json:"enrollment_token"`
}
type registerResponse struct {
	ID                  string `json:"id"`
	Name                string `json:"name"`
	IP                  string `json:"ip"`
	DeviceToken         string `json:"device_token"`
	ServerPublicKey     string `json:"server_public_key"`
	Endpoint            string `json:"endpoint"`
	AllowedIPs          string `json:"allowed_ips"`
	PersistentKeepalive int    `json:"persistent_keepalive"`
}
type errorResponse struct {
	Error string `json:"error"`
}

func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func bearer(r *http.Request) string {
	return strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
}
func (s *Server) admin(r *http.Request) bool { return bearer(r) == s.C.AdminToken }
func (s *Server) Handler() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		write(w, 200, map[string]string{"status": "ok", "service": "mesh-control"})
	})
	m.HandleFunc("POST /v1/devices/register", s.register)
	m.HandleFunc("POST /v1/devices/heartbeat", s.heartbeat)
	m.HandleFunc("GET /v1/devices", s.list)
	m.HandleFunc("DELETE /v1/devices/", s.delete)
	return logging(m)
}
func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	var q registerRequest
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&q) != nil {
		write(w, 400, errorResponse{"invalid JSON"})
		return
	}
	if q.EnrollmentToken != s.C.EnrollmentToken {
		write(w, 401, errorResponse{"invalid enrollment token"})
		return
	}
	d, e := s.S.Register(q.Name, q.PublicKey)
	if e != nil {
		write(w, 409, errorResponse{e.Error()})
		return
	}
	if e = wireguard.AddPeer(s.C.WGInterface, d.PublicKey, d.IP); e != nil {
		_, _, _ = s.S.Delete(d.ID)
		log.Printf("wireguard peer add failed: %v", e)
		write(w, 500, errorResponse{"failed to provision WireGuard peer"})
		return
	}
	write(w, 201, registerResponse{ID: d.ID, Name: d.Name, IP: d.IP, DeviceToken: d.Token, ServerPublicKey: s.C.WGServerPublicKey, Endpoint: s.C.PublicEndpoint, AllowedIPs: s.C.MeshCIDR, PersistentKeepalive: 25})
}
func (s *Server) heartbeat(w http.ResponseWriter, r *http.Request) {
	d, ok := s.S.ByToken(bearer(r))
	if !ok {
		write(w, 401, errorResponse{"unauthorized"})
		return
	}
	if e := s.S.Heartbeat(d.ID); e != nil {
		write(w, 500, errorResponse{"heartbeat failed"})
		return
	}
	write(w, 200, map[string]string{"status": "ok"})
}
func (s *Server) list(w http.ResponseWriter, r *http.Request) {
	if !s.admin(r) {
		write(w, 401, errorResponse{"unauthorized"})
		return
	}
	write(w, 200, map[string]any{"devices": s.S.List()})
}
func (s *Server) delete(w http.ResponseWriter, r *http.Request) {
	if !s.admin(r) {
		write(w, 401, errorResponse{"unauthorized"})
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/v1/devices/")
	d, ok, e := s.S.Delete(id)
	if e != nil {
		write(w, 500, errorResponse{"state update failed"})
		return
	}
	if !ok {
		write(w, 404, errorResponse{"device not found"})
		return
	}
	if e = wireguard.RemovePeer(s.C.WGInterface, d.PublicKey); e != nil {
		log.Printf("warning: remove peer %s: %v", id, e)
	}
	write(w, 200, map[string]string{"status": "revoked"})
}
func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}
