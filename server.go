package highmc

import "fmt"

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
