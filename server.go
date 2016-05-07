package highmc

import "fmt"

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
				s.players[req.player.Address.String()] = req.player
				req.ok <- nil
			} else {
				if _, ok := s.players[req.player.Address.String()]; !ok {
					req.ok <- fmt.Errorf("player does not exist")
					continue
				}
				delete(s.players, req.player.Address.String())
				req.ok <- nil
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
