package highmc

import (
	"fmt"
	"log"
	"sync/atomic"
	"unsafe"
)

// Server is a main server object.
type Server struct {
	*Router
	OpenSessions    map[string]struct{}
	Levels          map[string]*Level
	players         map[string]*player // Not goroutine-safe, so make it unexported.
	callbackRequest chan func(*player)
	close           chan struct{}
	registerRequest chan struct {
		player   *player
		ok       chan error
		register bool
	}
	broadcastRequest chan struct {
		packet MCPEPacket
		filter func(*player) bool
	}
}

// NewServer creates new server object.
func NewServer() *Server {
	s := new(Server)
	s.OpenSessions = make(map[string]struct{})
	s.Levels = map[string]*Level{
		defaultLvl: {Name: "dummy", Server: s},
	}
	s.players = make(map[string]*player)

	s.callbackRequest = make(chan func(*player), chanBufsize)
	s.registerRequest = make(chan struct {
		player   *player
		ok       chan error
		register bool // false: unregister
	}, chanBufsize)
	s.broadcastRequest = make(chan struct {
		packet MCPEPacket
		filter func(*player) bool
	}, chanBufsize)

	s.close = make(chan struct{})
	return s
}

// Start starts the server.
func (s *Server) Start() {
	go s.process()
}

func (s *Server) process() {
	for {
		select {
		case <-s.close:
			return
		case req := <-s.registerRequest:
			if req.register {
				if _, ok := s.players[req.player.Address.String()]; ok {
					req.ok <- fmt.Errorf("player exists with same address:port")
					continue
				}
				go req.player.once.Do(req.player.process)
				s.players[req.player.Address.String()] = req.player
				req.player.playerShown = make(map[uint64]struct{})
				for _, p := range s.players {
					if p.EntityID == req.player.EntityID { // player self
						continue
					}
					s.ShowPlayer(p, req.player)
					s.ShowPlayer(req.player, p)
				}
				req.ok <- nil
			} else {
				if _, ok := s.players[req.player.Address.String()]; !ok {
					req.ok <- fmt.Errorf("player does not exist")
					continue
				}
				delete(s.players, req.player.Address.String())
				req.ok <- nil
			}
		case req := <-s.broadcastRequest:
			for _, p := range s.players {
				if req.filter == nil || req.filter(p) {
					p.SendRequest <- req.packet
				}
			}
		}
	}
}

// RegisterPlayer attempts to register the player to server.
func (s *Server) RegisterPlayer(p *player) error {
	ok := make(chan error, 1)
	s.registerRequest <- struct {
		player   *player
		ok       chan error
		register bool
	}{
		p,
		ok,
		true,
	}
	res := <-ok
	if res != nil {
		return res
	}
	return nil
}

// UnregisterPlayer attempts to unregister the player from server.
func (s *Server) UnregisterPlayer(p *player) error {
	ok := make(chan error, 1)
	s.registerRequest <- struct {
		player   *player
		ok       chan error
		register bool
	}{
		p,
		ok,
		false,
	}
	res := <-ok
	if res != nil {
		return res
	}
	return nil
}

// BroadcastPacket broadcasts given MCPEPacket to all online players.
// If filter is not nil server will send packet to players only filter returns true.
func (s *Server) BroadcastPacket(pk MCPEPacket, filter func(*player) bool) {
	s.broadcastRequest <- struct {
		packet MCPEPacket
		filter func(*player) bool
	}{
		pk,
		filter,
	}
}

// Message broadcasts message to all players.
func (s *Server) Message(msg string) {
	s.BroadcastPacket(&Text{
		TextType: TextTypeRaw,
		Message:  msg,
	}, nil)
	log.Println("Broadcast> " + msg)
}

// ShowPlayer shows p to t.
func (s *Server) ShowPlayer(p, t *player) {
	x, y, z := unsafe.Pointer(&p.Position.X), unsafe.Pointer(&p.Position.Y), unsafe.Pointer(&p.Position.Z)
	t.SendRequest <- &AddPlayer{
		RawUUID:  p.UUID,
		Username: p.Username,
		EntityID: p.EntityID,
		X:        *(*float32)(atomic.LoadPointer(&x)),
		Y:        *(*float32)(atomic.LoadPointer(&y)),
		Z:        *(*float32)(atomic.LoadPointer(&z)),
		BodyYaw:  p.BodyYaw,
		Yaw:      p.Yaw,
		Pitch:    p.Pitch,
	}
	t.playerShown[p.EntityID] = struct{}{}
}

// RemovePlayer hides p from t.
func (s *Server) RemovePlayer(p, t *player) {
	if _, ok := t.playerShown[p.EntityID]; !ok {
		return
	}
	t.SendRequest <- &RemovePlayer{
		EntityID: p.EntityID,
		RawUUID:  p.UUID,
	}
	delete(t.playerShown, p.EntityID)
}
