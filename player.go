package highmc

import (
	"bytes"
	"fmt"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// PlayerCallback is a struct for delivering callbacks to other player goroutines;
// It is usually used to bypass race issues.
type PlayerCallback struct {
	Call func(*Player, interface{})
	Arg  interface{}
}

type chunkRequest struct {
	x, z int32
	wg   *sync.WaitGroup
}

// Player is a struct for handling/containing MCPE client specific things.
type Player struct {
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

	fastChunks     map[[2]int32]*Chunk
	fastChunkMutex *sync.Mutex
	chunkRadius    int32
	chunkRequest   chan chunkRequest
	chunkStop      chan struct{}
	chunkNotify    chan ChunkDelivery
	pending        map[[2]int32]time.Time

	inventory *PlayerInventory

	recvChan     chan *bytes.Buffer
	raknetChan   chan<- *EncapsulatedPacket
	callbackChan chan PlayerCallback
	updateTicker *time.Ticker

	loggedIn bool
	spawned  bool
	closed   bool
}

func (p *Player) process() {
	p.pending = make(map[[2]int32]time.Time)
	// resendTicker := time.NewTicker(time.Second * 3)
	// defer resendTicker.Stop()
	go p.updateChunk()
	for {
		select {
		case buf, ok := <-p.recvChan:
			if !ok {
				return
			}
			p.HandlePacket(buf)
		case callback := <-p.callbackChan:
			callback.Call(p, callback.Arg)
		case <-p.updateTicker.C:
			cx, cz := int32(p.Position.X)>>4, int32(p.Position.Z)>>4
			chunkHold := make(map[[2]int32]struct{})
			for ccx := cx - p.chunkRadius; ccx <= cx+p.chunkRadius; ccx++ {
				for ccz := cz - p.chunkRadius; ccz <= cz+p.chunkRadius; ccz++ {
					chunkHold[[2]int32{ccx, ccz}] = struct{}{}
				}
			}
			p.fastChunkMutex.Lock()
			for cc := range p.fastChunks {
				if _, ok := chunkHold[cc]; ok {
					delete(chunkHold, cc)
				} else {
					p.fastChunks[cc].Mutex().Lock()
					if p.fastChunks[cc].Refs <= 1 {
						p.Level.ChunkMutex.Lock()
						p.Level.CleanQueue[cc] = struct{}{}
						p.Level.ChunkMutex.Unlock()
					} else {
						p.fastChunks[cc].Refs--
					}
					p.fastChunks[cc].Mutex().Unlock()
					delete(p.fastChunks, cc)
				}
			}
			p.fastChunkMutex.Unlock()
			for cc := range chunkHold {
				if timeout, ok := p.pending[cc]; !ok || timeout.Before(time.Now()) {
					p.requestChunk(cc, nil)
				}
			}
		case c := <-p.chunkNotify:
			p.fastChunkMutex.Lock()
			if _, ok := p.fastChunks[[2]int32{c.X, c.Z}]; ok {
				p.fastChunkMutex.Unlock()
				break
			}
			p.Level.ChunkMutex.Lock()
			delete(p.Level.CleanQueue, [2]int32{c.X, c.Z})
			p.Level.ChunkMutex.Unlock()
			c.Chunk.Mutex().Lock()
			c.Chunk.Refs++
			c.Chunk.Mutex().Unlock()
			p.fastChunks[[2]int32{c.X, c.Z}] = c.Chunk
			p.fastChunkMutex.Unlock()
			delete(p.pending, [2]int32{c.X, c.Z})
			p.sendChunk(c)
			/*
				case <-resendTicker.C:
					for cx := int32(p.Position.X) - p.chunkRadius; cx <= int32(p.Position.X)+p.chunkRadius; cx++ {
						for cz := int32(p.Position.Z) - p.chunkRadius; cz <= int32(p.Position.Z)+p.chunkRadius; cz++ {
							p.requestChunk([2]int32{cx, cz})
						}
					}
			*/
		}
	}
}

// SendNearChunk sends chunks near the player in radius.
// This function should be run only on p.process goroutine, or RunAs().
func (p *Player) SendNearChunk(wg *sync.WaitGroup) {
	cx, cz := int32(p.Position.X)>>4, int32(p.Position.Z)>>4
	if wg != nil {
		wg.Add(int(p.chunkRadius*2+1) * int(p.chunkRadius*2+1))
	}
	for ccx := cx - p.chunkRadius; ccx <= cx+p.chunkRadius; ccx++ {
		for ccz := cz - p.chunkRadius; ccz <= cz+p.chunkRadius; ccz++ {
			p.requestChunk([2]int32{ccx, ccz}, wg)
		}
	}
}

// NOTE: Do NOT execute outside player process goroutine. pending map could be racy.
func (p *Player) requestChunk(cc [2]int32, wg *sync.WaitGroup) {
	go func(cc [2]int32) {
		p.chunkRequest <- chunkRequest{
			x:  cc[0],
			z:  cc[1],
			wg: wg,
		}
	}(cc)
	p.pending[cc] = time.Now().Add(time.Second * 5)
}

// NOTE: Do NOT execute. This is an internal function.
func (p *Player) updateChunk() {
	for {
		select {
		case <-p.chunkStop:
			return
		case req := <-p.chunkRequest:
			if c := p.getFastChunk(req.x, req.z); c != nil {
				p.chunkNotify <- ChunkDelivery{
					X:     req.x,
					Z:     req.z,
					Chunk: c,
				}
				if req.wg != nil {
					req.wg.Done()
				}
				continue
			}
			if ch := p.Level.CreateChunk(req.x, req.z); ch != nil {
				go func(cx, cz int32, wg *sync.WaitGroup, done <-chan struct{}) {
					<-done
					p.chunkNotify <- ChunkDelivery{
						X:     cx,
						Z:     cz,
						Chunk: p.Level.GetChunk(cx, cz),
					}
					if wg != nil {
						wg.Done()
					}
				}(req.x, req.z, req.wg, ch)
			}
		}
	}
}

// NOTE: Do NOT execute outside updateChunk goroutine. It could make data races.
func (p *Player) getFastChunk(cx, cz int32) *Chunk {
	p.fastChunkMutex.Lock()
	defer p.fastChunkMutex.Unlock()
	if c, ok := p.fastChunks[[2]int32{cx, cz}]; ok {
		return c
	}
	return p.Level.GetChunk(cx, cz)
}

// HandlePacket handles received MCPEPacket after raknet connection is established.
func (p *Player) HandlePacket(b *bytes.Buffer) (err error) {
	pid := ReadByte(b)
	var pk MCPEPacket
	if pk = GetPacket(pid); pk == nil {
		return
	}
	pk.Read(b)
	return p.handleDataPacket(pk)
}

func (p *Player) handleDataPacket(pk MCPEPacket) (err error) {
	switch pk.(type) {
	case *Login:
		pk := pk.(*Login)
		if p.loggedIn {
			return
		}
		// iteratorLock.Lock() FIXME
		if len(Sessions) >= int(atomic.LoadInt32(&MaxPlayers)) { // FIXME
			// iteratorLock.Unlock() FIXME
			p.disconnect("Server is full!")
			return
		}
		// iteratorLock.Unlock() FIXME
		p.Username = pk.Username

		ret := &PlayStatus{}
		if pk.Proto1 > MinecraftProtocol {
			ret.Status = LoginFailedServer
			p.SendPacket(ret)
			p.disconnect("Outdated server")
			return
		} else if pk.Proto1 < MinecraftProtocol {
			ret.Status = LoginFailedClient
			p.SendPacket(ret)
			p.disconnect("Outdated client")
			return
		}
		ret.Status = LoginSuccess
		p.SendPacket(ret)

		p.ID = pk.ClientID
		p.UUID = pk.RawUUID
		p.Secret = pk.ClientSecret
		p.SkinName = pk.SkinName
		p.Skin = pk.Skin
		var safeY float32
		wg := new(sync.WaitGroup)
		wg.Add(1)
		p.requestChunk([2]int32{0, 0}, wg)
		wg.Wait()
		func() {
			c := p.Level.GetChunk(0, 0)
			c.Mutex().Lock()
			defer func() {
				c.Mutex().Unlock()
				log.Println("Chunk unlocked")
			}()
			safeY = float32(c.GetHeightMap(0, 0) + 3)
		}()
		p.Position = Vector3{X: 0, Y: safeY, Z: 0}

		p.SendPacket(&StartGame{
			Seed:      0xffffffff, // -1
			Dimension: 0,
			Generator: 1, // 0: old, 1: infinite, 2: flat
			Gamemode:  1, // 0: Survival, 1: Creative
			EntityID:  0, // Player eid set to 0
			SpawnX:    0,
			SpawnY:    uint32(safeY),
			SpawnZ:    0,
			X:         0,
			Y:         safeY,
			Z:         0,
		})
		p.loggedIn = true

		p.inventory.Holder = p
		p.inventory.Init()
		// TODO: Send SetTime/SpawnPosition/Health/Difficulty Packets
		p.firstSpawn()

	case *Batch:
		pk := pk.(*Batch)
		for _, pp := range pk.Payloads {
			if err = p.HandlePacket(bytes.NewBuffer(pp)); err != nil {
				return
			}
		}

	case *Text:
		pk := pk.(*Text)
		if pk.TextType == TextTypeTranslation {
			return
		}
		Message(fmt.Sprintf("<%s> %s", p.Username, pk.Message))

	case *MovePlayer:
		pk := pk.(*MovePlayer)
		// log.Println("Player move:", pk.X, pk.Y, pk.Z, pk.Yaw, pk.BodyYaw, pk.Pitch)
		p.updateMove(pk)

	case *RemoveBlock:
		pk := pk.(*RemoveBlock)
		p.Level.OnRemoveBlock(p, int32(pk.X), int32(pk.Y), int32(pk.Z))

	case *UseItem:
		pk := pk.(*UseItem)
		px, py, pz := int32(pk.X), int32(pk.Y), int32(pk.Z)
		p.Level.OnUseItem(p, px, py, pz, pk.Face, pk.Item)
		//spew.Dump(pk)

	case *RequestChunkRadius:
		p.chunkRadius = int32(pk.(*RequestChunkRadius).Radius)

	case *ContainerSetSlot:
		//pk := pk.(*ContainerSetSlot)
		//spew.Dump(pk)

	case *Animate:
		pk := pk.(*Animate)
		pk.EntityID = p.EntityID
		p.BroadcastOthers(pk)

	default:
		// log.Println("0x" + hex.EncodeToString([]byte{pk.Pid()}) + "is unimplemented:")
		// spew.Dump(pk)
	}
	return
}

// SendMessage sends text to player.
func (p *Player) SendMessage(msg string) {
	p.SendPacket(&Text{
		TextType: TextTypeRaw,
		Message:  msg,
	})
}

//NOTE: This function is NOT goroutine-safe. Only for internal use.
func (p *Player) sendChunk(c ChunkDelivery) {
	c.Chunk.Mutex().RLock()
	i := &FullChunkData{
		ChunkX:  uint32(c.X),
		ChunkZ:  uint32(c.Z),
		Order:   OrderLayered,
		Payload: c.Chunk.FullChunkData(),
	}
	c.Chunk.Mutex().RUnlock()
	p.SendCompressed(i)
}

// ShowPlayer shows given player struct to player.
func (p *Player) ShowPlayer(player *Player) {
	if p.IsVisible(player) || p.IsSelf(player) {
		return
	}
	p.SendPacket(&AddPlayer{
		RawUUID:  player.UUID,
		Username: player.Username,
		EntityID: player.EntityID,
		X:        player.Position.X,
		Y:        player.Position.Y,
		Z:        player.Position.Z,
		SpeedX:   0,
		SpeedY:   0,
		SpeedZ:   0,
		BodyYaw:  player.BodyYaw,
		Yaw:      player.Yaw,
		Pitch:    player.Pitch,
	})
	p.playerShown[player.EntityID] = struct{}{}
}

// HidePlayer hides given player struct from player.
func (p *Player) HidePlayer(player *Player) {
	if !p.IsVisible(player) || p.IsSelf(player) {
		return
	}
	p.SendPacket(&RemovePlayer{
		EntityID: player.EntityID,
		RawUUID:  player.UUID,
	})
	delete(p.playerShown, player.EntityID)
}

// IsVisible determines if the player can see given player struct
func (p *Player) IsVisible(player *Player) bool {
	_, ok := p.playerShown[player.EntityID]
	return ok
}

// IsSelf determines if given player is the player self
func (p *Player) IsSelf(player *Player) bool {
	return p.EntityID == player.EntityID
}

func (p *Player) updateMove(pk *MovePlayer) {
	p.Position.X, p.Position.Y, p.Position.Z = pk.X, pk.Y, pk.Z
	p.Yaw, p.BodyYaw, p.Pitch = pk.Yaw, pk.BodyYaw, pk.Pitch

	go BroadcastCallback(PlayerCallback{
		Call: func(pl *Player, arg interface{}) {
			if pl.IsVisible(p) {
				pl.SendPacket(&MoveEntity{
					EntityIDs: []uint64{p.EntityID},
					EntityPos: [][6]float32{{
						pk.X,
						pk.Y,
						pk.Z,
						pk.BodyYaw,
						pk.Yaw,
						pk.Pitch,
					}},
				})
			}
		},
	})
}

func (p *Player) firstSpawn() {
	if p.spawned {
		return
	}

	BroadcastCallback(PlayerCallback{
		Call: func(player *Player, arg interface{}) {
			player.ShowPlayer(p)
			player.SendPacket(&PlayerList{
				Type: PlayerListAdd,
				PlayerEntries: []PlayerListEntry{{
					RawUUID:  p.UUID,
					EntityID: p.EntityID,
					Username: p.Username,
					Skin:     p.Skin,
				}},
			})
		},
	})

	var entries []PlayerListEntry
	AsPlayers(func(pl *Player) {
		p.ShowPlayer(pl)
		entries = append(entries, PlayerListEntry{
			RawUUID:  pl.UUID,
			EntityID: pl.EntityID,
			Username: pl.Username,
			Skin:     pl.Skin,
		})
	})

	wg := new(sync.WaitGroup)
	p.SendNearChunk(wg)

	go func() {
		wg.Wait()

		p.SendPacket(&PlayStatus{
			Status: PlayerSpawn,
		})

		SpawnPlayer(p)
		p.RunAs(PlayerCallback{
			Call: func(pl *Player, arg interface{}) {
				p.spawned = true
				Message(p.Username + " joined")
				log.Println(p.Username + " joined the game")
				p.SendMessage("Hello, this is lav7 test server!")
			},
		})
	}()

	p.SendPacket(&PlayerList{
		Type:          PlayerListAdd,
		PlayerEntries: entries,
	})
}

// Kick kicks player from server.
func (p *Player) Kick(reason string) {
	p.disconnect("Kicked: " + reason)
}

func (p *Player) disconnect(msg string) {
	p.SendDirect(&Disconnect{
		Message: msg,
	})

	// SessionLock.Lock() FIXME
	s, ok := Sessions[p.Address.String()]
	// SessionLock.Unlock() FIXME
	if ok {
		s.Close(msg)
	}
}

// BroadcastOthers broadcasts MCPEPacket except player self.
func (p *Player) BroadcastOthers(pk MCPEPacket) {
	AsPlayers(func(pl *Player) {
		if !pl.IsSelf(p) {
			pl.SendPacket(pk)
		}
	})
}

// SendPacket sends given MCPEPacket to client.
func (p *Player) SendPacket(pk MCPEPacket) {
	buf := bytes.NewBuffer([]byte{0x8e, pk.Pid()})
	Write(buf, pk.Write().Bytes())
	p.Send(buf)
}

// SendDirect sends given MCPEPacket without passing to raknetChan channel.
func (p *Player) SendDirect(pk MCPEPacket) {
	buf := bytes.NewBuffer([]byte{0x8e, pk.Pid()})
	Write(buf, pk.Write().Bytes())

	ep := new(EncapsulatedPacket)
	ep.Reliability = 2
	ep.Buffer = buf

	// SessionLock.Lock() FIXME
	s, ok := Sessions[p.Address.String()]
	// SessionLock.Unlock() FIXME

	if ok {
		s.SendEncapsulated(ep)
	}
}

// SendCompressed sends packed BatchPacket with given MCPEPackets.
func (p *Player) SendCompressed(pks ...MCPEPacket) {
	batch := &Batch{
		Payloads: make([][]byte, len(pks)),
	}
	for i, pk := range pks {
		batch.Payloads[i] = append([]byte{pk.Pid()}, pk.Write().Bytes()...)
	}
	p.SendPacket(batch)
}

// RunAs runs given callback on the player's goroutine.
// You should use this if you need access to other players' fields.
func (p *Player) RunAs(callback PlayerCallback) {
	p.callbackChan <- callback
}

// Send sends bytes buffer to client.
// Do not use this method for sending MCPEPacket to client, this is an internal function.
func (p *Player) Send(buf *bytes.Buffer) {
	ep := new(EncapsulatedPacket)
	ep.Reliability = 2
	ep.Buffer = buf
	p.raknetChan <- ep
}
