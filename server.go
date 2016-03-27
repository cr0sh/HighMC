package highmc

import (
	"fmt"
	"log"
	"net"
	"os"
	"sync/atomic"
)

// Server is a main server object.
type Server struct {
	*Router
	OpenSessions    map[string]struct{}
	Levels          map[string]*Level
	callbackRequest chan func(*Player)
	registerRequest chan struct {
		player *Player
		ok     chan string
	}
}

// NewServer creates new server object.
func NewServer() *Server {
	s := new(Server)
	s.OpenSessions = make(map[string]struct{})
	s.Levels = map[string]*Level{
		defaultLvl: {Name: "dummy", Server: s},
	}
	s.callbackRequest = make(chan func(*Player), chanBufsize)
	s.registerRequest = make(chan struct {
		player *Player
		ok     chan string
	}, chanBufsize)
	return s
}

// HeartBeat processes requests to server.
func (s *Server) HeartBeat() {
	for {
		select {
		case callback := <-s.callbackRequest:
			for addr := range s.OpenSessions {
				callback(s.Sessions[addr].Player)
			}
		case req := <-s.registerRequest:
			req.ok <- s.RegisterPlayer(req.player)
		}
	}
}

// RegisterPlayer registers player to the server.
func (s *Server) RegisterPlayer(player *Player) string {
	identifier := player.Address.String()
	if _, ok := s.OpenSessions[identifier]; ok {
		fmt.Println("Duplicate authentication from", identifier)
		s.Sessions[identifier].Player.disconnect("Logged in from another location")
	}
	if len(s.OpenSessions) >= int(atomic.LoadInt32(&MaxPlayers)) {
		return "Server is full"
	}
	s.OpenSessions[player.Address.String()] = struct{}{}
	return ""
}

// UnregisterPlayer removes player from server.
func UnregisterPlayer(addr *net.UDPAddr) error {
	/* FIXME
	identifier := addr.String()
	iteratorLock.Lock()
	if p, ok := Sessions[identifier]; ok {
		iteratorLock.Unlock()
		p.updateTicker.Stop()
		p.chunkStop <- struct{}{}
		AsPlayers(func(pl *Player) {
			if p.EntityID == pl.EntityID {
				return
			}
			pl.HidePlayer(p) //FIXME: semms not working
		})
		iteratorLock.Lock()
		delete(Sessions, identifier)
		iteratorLock.Unlock()
		atomic.AddInt32(&OnlineSessions, -1)
		if p.loggedIn {
			Message(p.Username + " disconnected")
		}
		return nil
	}
	iteratorLock.Unlock()
	return fmt.Errorf("Tried to remove nonexistent player: %v", addr)
	*/
	return nil
}

// AsPlayers executes given callback with every online players.
//
// Warning: callbacks are executed in separate, copied map of lav7.Sessions. Callbacks can run with disconnected player.
func (s *Server) AsPlayers(callback func(*Player)) {
	s.callbackRequest <- callback
}

// BroadcastCallback is same as AsPlayers(RunAs())
func (s *Server) BroadcastCallback(callback PlayerCallback) {
	s.AsPlayers(func(p *Player) {
		p.RunAs(callback)
	})
}

// Message broadcasts message, and logs to console.
func (s *Server) Message(msg string) {
	s.AsPlayers(func(pl *Player) {
		pl.SendMessage(msg)
	})
	log.Println(msg)
}

// SpawnPlayer shows given player to all players, except given player itself.
func (s *Server) SpawnPlayer(player *Player) {
	s.AsPlayers(func(p *Player) {
		if p.spawned && p.EntityID != player.EntityID {
			p.ShowPlayer(player)
		}
	})
}

// BroadcastPacket sends given packet to all online players.
func (s *Server) BroadcastPacket(pk MCPEPacket) {
	for addr := range s.OpenSessions {
		s.Sessions[addr].Player.SendPacket(pk)
	}
}

// GetLevel returns level reference with given name if exists, or nil.
func (s *Server) GetLevel(name string) *Level {
	if l, ok := s.Levels[name]; ok {
		return l
	}
	return nil
}

// GetDefaultLevel returns default level reference.
func (s *Server) GetDefaultLevel() *Level {
	return s.Levels[defaultLvl]
}

// Stop stops entire server.
func (s *Server) Stop(reason string) {
	if reason == "" {
		reason = "no reason"
	}
	log.Println("Stopping server: " + reason)
	s.AsPlayers(func(p *Player) { p.Kick("Server stop: " + reason) })
	for _, l := range s.Levels {
		l.Save()
	}
	os.Exit(0)
}
