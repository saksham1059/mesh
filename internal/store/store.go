package store

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Device struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	PublicKey string    `json:"public_key"`
	IP        string    `json:"ip"`
	Token     string    `json:"token"`
	CreatedAt time.Time `json:"created_at"`
	LastSeen  time.Time `json:"last_seen"`
}
type diskState struct {
	Devices map[string]Device `json:"devices"`
}
type Store struct {
	mu      sync.Mutex
	path    string
	devices map[string]Device
	network *net.IPNet
	next    uint32
}

func New(path, cidr string) (*Store, error) {
	_, n, e := net.ParseCIDR(cidr)
	if e != nil {
		return nil, e
	}
	s := &Store{path: path, devices: map[string]Device{}, network: n, next: 2}
	b, e := os.ReadFile(path)
	if e == nil {
		var d diskState
		if json.Unmarshal(b, &d) == nil && d.Devices != nil {
			s.devices = d.Devices
		}
	} else if !os.IsNotExist(e) {
		return nil, e
	}
	for _, d := range s.devices {
		ip := net.ParseIP(d.IP).To4()
		if ip != nil {
			v := uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3])
			if v >= s.base()+s.next {
				s.next = v - s.base() + 1
			}
		}
	}
	return s, nil
}
func (s *Store) base() uint32 {
	b := s.network.IP.To4()
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
}
func randHex(n int) (string, error) {
	b := make([]byte, n)
	if _, e := rand.Read(b); e != nil {
		return "", e
	}
	return hex.EncodeToString(b), nil
}
func (s *Store) save() error {
	if e := os.MkdirAll(filepath.Dir(s.path), 0700); e != nil {
		return e
	}
	b, e := json.MarshalIndent(diskState{Devices: s.devices}, "", "  ")
	if e != nil {
		return e
	}
	tmp := s.path + ".tmp"
	if e = os.WriteFile(tmp, b, 0600); e != nil {
		return e
	}
	return os.Rename(tmp, s.path)
}
func (s *Store) Register(name, pub string) (Device, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, d := range s.devices {
		if d.PublicKey == pub {
			return d, fmt.Errorf("device public key is already registered")
		}
	}
	if name == "" || pub == "" {
		return Device{}, fmt.Errorf("name and public key are required")
	}
	bits, _ := s.network.Mask.Size()
	hostBits := 32 - bits
	max := uint32(1<<hostBits) - 1
	for s.next < max {
		v := s.base() + s.next
		s.next++
		ip := net.IPv4(byte(v>>24), byte(v>>16), byte(v>>8), byte(v)).String()
		used := false
		for _, d := range s.devices {
			if d.IP == ip {
				used = true
				break
			}
		}
		if used {
			continue
		}
		id, e := randHex(8)
		if e != nil {
			return Device{}, e
		}
		tok, e := randHex(32)
		if e != nil {
			return Device{}, e
		}
		now := time.Now().UTC()
		d := Device{ID: id, Name: name, PublicKey: pub, IP: ip, Token: tok, CreatedAt: now, LastSeen: now}
		s.devices[id] = d
		if e = s.save(); e != nil {
			delete(s.devices, id)
			return Device{}, e
		}
		return d, nil
	}
	return Device{}, fmt.Errorf("mesh address pool exhausted")
}
func (s *Store) ByID(id string) (Device, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.devices[id]
	return d, ok
}
func (s *Store) ByToken(t string) (Device, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, d := range s.devices {
		if d.Token == t {
			return d, true
		}
	}
	return Device{}, false
}
func (s *Store) Heartbeat(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.devices[id]
	if !ok {
		return fmt.Errorf("unknown device")
	}
	d.LastSeen = time.Now().UTC()
	s.devices[id] = d
	return s.save()
}
func (s *Store) List() []Device {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Device, 0, len(s.devices))
	for _, d := range s.devices {
		d.Token = ""
		out = append(out, d)
	}
	return out
}
func (s *Store) Delete(id string) (Device, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.devices[id]
	if !ok {
		return Device{}, false, nil
	}
	delete(s.devices, id)
	return d, true, s.save()
}
