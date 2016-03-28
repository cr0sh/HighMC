package highmc

import (
	"bytes"
	"encoding/hex"
	"log"
	"net"
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
	Address  *net.UDPAddr
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

	Session *Session

	loggedIn bool
	spawned  bool
	closed   bool
}

// NewPlayer creates new player struct.
func NewPlayer(session *Session) *Player {
	p := new(Player)
	p.Session = session
	// p.Level = p.Server.GetDefaultLevel()
	p.EntityID = atomic.AddUint64(&lastEntityID, 1)
	p.playerShown = make(map[uint64]struct{})

	p.callbackChan = make(chan PlayerCallback, 128)
	p.updateTicker = time.NewTicker(time.Millisecond * 500)

	p.fastChunks = make(map[[2]int32]*Chunk)
	p.fastChunkMutex = new(sync.Mutex)
	p.chunkRadius = 16
	p.chunkStop = make(chan struct{}, 1)
	p.chunkRequest = make(chan chunkRequest, 256)
	p.chunkNotify = make(chan ChunkDelivery, 16)

	p.inventory = new(PlayerInventory)
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
	var handle Handleable
	if handle, ok = pk.(Handleable); !ok {
		return // There is no handler for the packet
	}
	pk.Read(buf)
	if err := pk.Handle(p); err != nil {
		log.Println("Error while handling packet:", err)
	}
}
