package highmc

import (
	"bytes"
	"io"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// PlayerCallback is a struct for delivering callbacks to other player goroutines;
// It is usually used to bypass race issues.
type PlayerCallback struct {
	Call func(*player)
}

type chunkResult struct {
	cx, cz int32
	chunk  *Chunk
}

// Player is a struct for handling/containing MCPE client specific things.
type player struct {
	*session
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

	SendRequest           chan MCPEPacket
	SendCompressedRequest chan []MCPEPacket

	chunkUpdate *time.Ticker
	chunkResult chan chunkResult

	loggedIn bool
	spawned  bool
	once     *sync.Once
}

// NewPlayer creates new player struct.
func NewPlayer(session *session) *player {
	p := new(player)
	p.session = session
	// p.Level = p.Server.GetDefaultLevel()
	p.EntityID = atomic.AddUint64(&lastEntityID, 1)

	p.SendRequest = make(chan MCPEPacket, chanBufsize)
	p.SendCompressedRequest = make(chan []MCPEPacket, chanBufsize)
	p.inventory = new(PlayerInventory)

	p.once = new(sync.Once)
	return p
}

// HandlePacket handles MCPE data packet.
func (p *player) HandlePacket(buf *bytes.Buffer) error {
	head := ReadByte(buf)
	pk := GetMCPEPacket(head)
	if pk == nil {
		log.Printf("[!] Unexpected packet head: 0x%02x", head)
		return nil
	}
	var ok bool
	var handler Handleable
	if handler, ok = pk.(Handleable); !ok {
		return nil // There is no handler for the packet
	}
	handler.Read(buf)
	if err := handler.Handle(p); err != nil {
		log.Println("Error while handling packet:", err)
		return err
	}
	Pool.Recycle(buf)
	return nil
}

func (p *player) firstSpawn() {
	chunk := new(Chunk)
	for x := byte(0); x < byte(16); x++ {
		for z := byte(0); z < byte(16); z++ {
			for y := byte(0); y < byte(56); y++ {
				chunk.SetBlock(x, y, z, Dirt.Block())
			}
			chunk.SetBlock(x, 56, z, Grass.Block())
			chunk.SetBiomeColor(x, z, 20, 128, 10)
		}
	}
	payload := chunk.FullChunkData()
	radius := int32(3)
	for cx := int32(0) - radius; cx <= radius; cx++ {
		for cz := int32(0) - radius; cz <= radius; cz++ {
			p.SendCompressed(&FullChunkData{
				ChunkX:  uint32(cx),
				ChunkZ:  uint32(cz),
				Order:   OrderLayered,
				Payload: payload,
			})
		}
	}
	p.SendPacket(&AdventureSettings{
		Flags:            0,
		UserPermission:   0x02,
		GlobalPermission: 0x02,
	})
	p.SendPacket(&PlayStatus{
		Status: PlayerSpawn,
	})
	log.Println("PlayStatus PlayerSpawn")
}

func (p *player) process() {
	p.chunkUpdate = time.NewTicker(time.Millisecond * 200)
	p.chunkResult = make(chan chunkResult, chanBufsize)
	// chunkReq := make(chan [2]int32, chanBufsize)
	for {
		select {
		case <-p.closed:
			if err := p.Server.UnregisterPlayer(p); err != nil {
				log.Println("Error while unregistering player:", err)
			}
			return
		case res := <-p.chunkResult:
			if res.chunk == nil {
				log.Println("Chunk gen on", res.cx, res.cz, "failed")
				continue
			}
			// TODO: mark sent chunks
			p.SendCompressed(&FullChunkData{
				ChunkX:  uint32(res.cx),
				ChunkZ:  uint32(res.cz),
				Order:   OrderLayered,
				Payload: res.chunk.FullChunkData(),
			})
		case pk := <-p.SendRequest:
			p.SendPacket(pk)
		case pks := <-p.SendCompressedRequest:
			p.SendCompressed(pks...)

			// case <-p.chunkUpdate.C:
			// 	    p.updateChunk()
		}
	}
}

func (p *player) updateChunk() {
	// TODO
}

// BroadcastOthers sends message to all other players.
func (p *player) BroadcastOthers(msg string) {
	p.Server.BroadcastPacket(&Text{
		TextType: TextTypeRaw,
		Message:  msg,
	}, func(t *player) bool {
		return t.EntityID != p.EntityID
	})
}

// Disconnect kicks player from the server.
// Arguments are dynamic. Player.Disconnect(ToSend, ToLog) will send ToSend string to client, and log ToLog to logger.
// If you supply nothing, or "" for ToSend, it'll be set to default.
// Similarly, if you supply "" or nothing for ToLog, it'll be same as ToSend.
func (p *player) Disconnect(opts ...string) {
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
	p.SendPacket(&Disconnect{
		Message: msg,
	})
	p.BroadcastOthers(p.Username + " quit the game")
	p.Close(log)
}

// SendCompressed sends packed BatchPacket with given packets.
func (p *player) SendCompressed(pks ...MCPEPacket) {
	batch := &Batch{
		Payloads: make([][]byte, len(pks)),
	}
	for i, pk := range pks {
		batch.Payloads[i] = pk.Write().Bytes()
	}
	p.SendPacket(batch)
}

func (p *player) SendPacket(pk MCPEPacket) {
	buf := pk.Write()
	p.SendRaw(buf)
	Pool.Recycle(buf)
}

// SendRaw sends raw bytes buffer to client.
func (p *player) SendRaw(buf *bytes.Buffer) {
	ep := new(EncapsulatedPacket)
	ep.Reliability = 2
	ep.Buffer = Pool.NewBuffer([]byte{0x8e})
	io.Copy(ep.Buffer, buf)
	p.EncapsulatedChan <- ep
}
