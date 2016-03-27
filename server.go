package highmc

import (
	"bytes"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// RegisterPlayer registers player to the server and returns packet handler function for it.
func RegisterPlayer(addr *net.UDPAddr) (handlerChan chan<- *bytes.Buffer) {
	identifier := addr.String()
	if _, ok := Sessions[identifier]; ok {
		fmt.Println("Duplicate authentication from", addr)
		// Sessions[identifier].disconnect("Logged in from another location") FIXME
	}

	p := new(Player)
	p.Address = addr
	p.Level = GetDefaultLevel()
	p.EntityID = atomic.AddUint64(&lastEntityID, 1)
	p.playerShown = make(map[uint64]struct{})

	ch := make(chan *bytes.Buffer, 64)
	p.recvChan = ch
	p.raknetChan = Sessions[identifier].PlayerChan
	p.callbackChan = make(chan PlayerCallback, 128)
	p.updateTicker = time.NewTicker(time.Millisecond * 500)

	p.fastChunks = make(map[[2]int32]*Chunk)
	p.fastChunkMutex = new(sync.Mutex)
	p.chunkRadius = 16
	p.chunkStop = make(chan struct{}, 1)
	p.chunkRequest = make(chan chunkRequest, 256)
	p.chunkNotify = make(chan ChunkDelivery, 16)

	p.inventory = new(PlayerInventory)
	/* FIXME
		iteratorLock.Lock()
		Sessions[identifier] = p
		iteratorLock.Unlock()
	    atomic.AddInt32(&OnlineSessions, 1)
	*/
	go p.process()
	return ch
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
func AsPlayers(callback func(*Player)) {
	/*
		iteratorLock.Lock() FIXME
		pm := getMapCopy()
		iteratorLock.Unlock()
		for _, p := range pm {
			callback(p)
		}
	*/
}

// AsPlayersAsync is similar to AsPlayers, buf spawns new goroutine for each players.
// It returns sync.WaitGroup struct to synchronize with callbacks.
//
// Warning: this could be a lot of overhead. Use with caution.
func AsPlayersAsync(callback func(*Player)) *sync.WaitGroup {
	// iteratorLock.Lock() FIXME
	pm := getMapCopy()
	// iteratorLock.Unlock()
	wg := new(sync.WaitGroup)
	for _, p := range pm {
		wg.Add(1)
		go func(pp *Player, w *sync.WaitGroup) {
			callback(pp)
			w.Done()
		}(p, wg)
	}
	return wg
}

// AsPlayersError is similar to AsPlayers, but breaks iteration if callback returns error
func AsPlayersError(callback func(*Player) error) error {
	// iteratorLock.Lock() FIXME
	pm := getMapCopy()
	// iteratorLock.Unlock() FIXME
	for _, p := range pm {
		if err := callback(p); err != nil {
			return err
		}
	}
	return nil
}

// BroadcastCallback is same as AsPlayers(RunAs())
func BroadcastCallback(callback PlayerCallback) {
	AsPlayers(func(p *Player) {
		p.RunAs(callback)
	})
}

func getMapCopy() map[string]*Player {
	m := make(map[string]*Player)
	/* FIXME
	for k := range Sessions {
		m[k] = Sessions[k]
	}
	*/
	return m
}

// Message broadcasts message, and logs to console.
func Message(msg string) {
	AsPlayers(func(pl *Player) {
		pl.SendMessage(msg)
	})
	log.Println(msg)
}

// SpawnPlayer shows given player to all players, except given player itself.
func SpawnPlayer(player *Player) {
	AsPlayers(func(p *Player) {
		if p.spawned && p.EntityID != player.EntityID {
			p.ShowPlayer(player)
		}
	})
}

// BroadcastPacket sends given packet to all online players.
func BroadcastPacket(pk MCPEPacket) {
	/* FIXME
	for _, p := range Sessions {
		p.SendPacket(pk)
	}
	*/
}

// GetLevel returns level reference with given name if exists, or nil.
func GetLevel(name string) *Level {
	if l, ok := levels[name]; ok {
		return l
	}
	return nil
}

// GetDefaultLevel returns default level reference.
func GetDefaultLevel() *Level {
	return levels[defaultLvl]
}

// Stop stops entire server.
func Stop(reason string) {
	if reason == "" {
		reason = "no reason"
	}
	log.Println("Stopping server: " + reason)
	AsPlayers(func(p *Player) { p.Kick("Server stop: " + reason) })
	for _, l := range levels {
		l.Save()
	}
	os.Exit(0)
}
