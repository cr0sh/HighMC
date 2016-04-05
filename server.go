package highmc

import "fmt"

// Server is a main server object.
type Server struct {
	*Router
	OpenSessions    map[string]struct{}
	Levels          map[string]*Level
	players         map[string]*Player // Not goroutine-safe, so make it unexported.
	callbackRequest chan func(*Player)
	close           chan struct{}
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
	s.players = make(map[string]*Player)
	s.callbackRequest = make(chan func(*Player), chanBufsize)
	s.registerRequest = make(chan struct {
		player *Player
		ok     chan string
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
			if _, ok := s.players[req.player.Address.String()]; ok {
				req.ok <- "player exists with same address:port"
				continue
			}
			s.players[req.player.Address.String()] = req.player
			req.ok <- ""
		}
	}
}

// RegisterPlayer attempts to register the player to server.
func (s *Server) RegisterPlayer(p *Player) error {
	ok := make(chan string, 1)
	s.registerRequest <- struct {
		player *Player
		ok     chan string
	}{
		p,
		ok,
	}
	res := <-ok
	if res != "" {
		return fmt.Errorf(res)
	}
	return nil
}
