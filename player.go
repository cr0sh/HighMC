package highmc

import (
	"bytes"
	"encoding/hex"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// PlayerCallback is a struct for delivering callbacks to other player goroutines;
// It is usually used to bypass race issues.
type PlayerCallback struct {
	Call func(*Player)
}

type chunkRequest struct {
	x, z int32
	wg   *sync.WaitGroup
}

// Player is a struct for handling/containing MCPE client specific things.
type Player struct {
	*Session
	Username string
	ID       uint64
	UUID     [16]byte
	Secret   string
	EntityID uint64
	Skin     []byte
	SkinName string

	Position            Vector3
	Level               *Level
	Yaw, BodyYaw, Pitch float32

	playerShown map[uint64]struct{}

	inventory *PlayerInventory

	SendRequest chan MCPEPacket

	loggedIn bool
	spawned  bool
}

// NewPlayer creates new player struct.
func NewPlayer(session *Session) *Player {
	p := new(Player)
	p.Session = session
	// p.Level = p.Server.GetDefaultLevel()
	p.EntityID = atomic.AddUint64(&lastEntityID, 1)
	p.playerShown = make(map[uint64]struct{})

	p.updateTicker = time.NewTicker(time.Millisecond * 500)

	p.SendRequest = make(chan MCPEPacket, chanBufsize)
	p.inventory = new(PlayerInventory)
	go p.process()
	return p
}

// HandlePacket handles MCPE data packet.
func (p *Player) HandlePacket(buf *bytes.Buffer) {
	head := ReadByte(buf)
	pk := GetMCPEPacket(head)
	if pk == nil {
		log.Println("[!] Unexpected packet head:", hex.EncodeToString([]byte{head}))
		return
	}
	var ok bool
	var handler Handleable
	if handler, ok = pk.(Handleable); !ok {
		return // There is no handler for the packet
	}
	handler.Read(buf)
	if err := handler.Handle(p); err != nil {
		log.Println("Error while handling packet:", err)
	}
}

func (p *Player) process() {
	for {
		select {
		case <-p.closed:
			return
		case pk := <-p.SendRequest:
			p.Send(pk.Write())
		}
	}
}

// Disconnect kicks player from the server.
// Arguments are dynamic. Player.Disconnect(ToSend, ToLog) will send ToSend string to client, and log ToLog to logger.
// If you supply nothing, or "" for ToSend, it'll be set to default.
// Similarly, if you supply "" or nothing for ToLog, it'll be same as ToSend.
func (p *Player) Disconnect(opts ...string) {
	var msg, log string
	if len(opts) == 0 || opts[0] == "" {
		msg = "Generic reason"
	} else {
		msg = opts[0]
	}
	if len(opts) < 2 || opts[1] == "" {
		log = msg
	} else {
		log = opts[1]
	}
	pk := &Disconnect{
		Message: msg,
	}
	p.Send(pk.Write())

	p.Close(log)
}

// Send sends raw bytes buffer to client.
func (p *Player) Send(buf *bytes.Buffer) {
	ep := new(EncapsulatedPacket)
	ep.Reliability = 2
	ep.Buffer = buf
	p.SendEncapsulated(ep)
}
