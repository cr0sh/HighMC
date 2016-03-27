package highmc

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
