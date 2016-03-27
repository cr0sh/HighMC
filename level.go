package highmc

import (
	"fmt"
	"log"
	"sync"
	"time"
)

const tickDuration = time.Millisecond * 50

type genRequest struct {
	done   chan<- struct{}
	cx, cz int32
}

type updateRequest struct {
	x, y, z int32
	records []BlockRecord
	done    chan<- []BlockRecord
}

// Level is a struct for processing MCPE worlds.
type Level struct {
	LevelProvider
	Name string

	ChunkMap   map[[2]int32]*Chunk
	ChunkMutex *sync.Mutex
	genTask    chan genRequest
	Gen        func(int32, int32) *Chunk

	CleanQueue map[[2]int32]struct{}

	UpdateRequest chan updateRequest

	Server *Server
	Ticker *time.Ticker
	Stop   chan struct{}
}

// Init initializes the level.
func (lv *Level) Init(pv LevelProvider, numWorkers int) {
	lv.LevelProvider = pv
	lv.ChunkMap = make(map[[2]int32]*Chunk)
	lv.ChunkMutex = new(sync.Mutex)
	// lv.Ticker = time.NewTicker(tickDuration)
	lv.UpdateRequest = make(chan updateRequest, 1024)
	lv.Stop = make(chan struct{}, 1)
	lv.genTask = make(chan genRequest, 512)
	lv.CleanQueue = make(map[[2]int32]struct{})
	pv.Init(lv.Name)
	log.Printf("* level: generating %d workers for chunk gen", numWorkers)
	for i := 0; i < numWorkers; i++ {
		go lv.genWorker()
	}
}

// Process fires tick periodically, and processes block update schedules.
func (lv *Level) Process() {
	var m map[[3]int32]struct{}
	for {
		select {
		// case <-lv.Ticker.C:
		// 	lv.tick()
		case req := <-lv.UpdateRequest:
			m = make(map[[3]int32]struct{})
			req.done <- append(req.records, lv.updateBlock(req.x, req.y, req.z, &m)...)
		case <-lv.Stop:
			break
		}
	}
}

func (lv *Level) tick() {
	// TODO
}

func (lv *Level) genWorker() {
	for task := range lv.genTask {
		c := lv.Gen(task.cx, task.cz)
		lv.ChunkMutex.Lock()
		if _, ok := lv.ChunkMap[[2]int32{task.cx, task.cz}]; ok {
			lv.ChunkMutex.Unlock()
			task.done <- struct{}{}
			continue
		}
		lv.SetChunk(task.cx, task.cz, c)
		lv.ChunkMutex.Unlock()
		task.done <- struct{}{}
	}
}

// OnUseItem handles UseItemPacket and determines position to update block position.
func (lv *Level) OnUseItem(p *Player, x, y, z int32, face byte, item *Item) {
	if !item.IsBlock() {
		return
	}
	switch face {
	case SideDown:
		y--
	case SideUp:
		y++
	case SideNorth:
		z--
	case SideSouth:
		z++
	case SideWest:
		x--
	case SideEast:
		x++
	case 255:
		return
	}
	if y > 127 {
		return
	}
	if f := lv.GetBlock(x, y, z); f == 0 {
		block := item.Block()
		if lv.placeHook(x, y, z, face, &block) {
			goto canceled
		}
		lv.Set(x, y, z, block)
		records := []BlockRecord{
			{
				X:     uint32(x),
				Y:     byte(y),
				Z:     uint32(z),
				Block: block,
				Flags: UpdateAllPriority,
			},
		}
		go func(w <-chan []BlockRecord) {
			lv.Server.BroadcastPacket(&UpdateBlock{
				BlockRecords: <-w,
			})
		}(lv.requestUpdate(x, y, z, records))
		return
	}
	// p.SendMessage(fmt.Sprintf("Block %d(%s) already exists on x:%d, y:%d, z: %d", f, ID(f), x, y, z))
canceled:
	p.SendPacket(&UpdateBlock{
		BlockRecords: []BlockRecord{
			{
				X:     uint32(x),
				Y:     byte(y),
				Z:     uint32(z),
				Block: lv.Get(x, y, z),
				Flags: UpdateAllPriority,
			},
		},
	})

}

// OnRemoveBlock handles RemoveBlockPacket.
func (lv *Level) OnRemoveBlock(p *Player, x, y, z int32) {
	lv.Set(x, y, z, Block{ID: byte(Air)})
	records := []BlockRecord{
		{
			X:     uint32(x),
			Y:     byte(y),
			Z:     uint32(z),
			Block: Block{ID: byte(Air)},
			Flags: UpdateAllPriority,
		},
	}
	go func(w <-chan []BlockRecord) {
		records := <-w
		lv.Server.BroadcastPacket(&UpdateBlock{
			BlockRecords: records,
		})
	}(lv.requestUpdate(x, y, z, records))
}

func (lv *Level) placeHook(x, y, z int32, face byte, block *Block) bool { // should return true if canceled
	if face > 5 {
		return true
	}
	switch block.ID {
	case byte(Torch):
		block.Meta = [...]byte{0, 5, 4, 3, 2, 1}[face]
	}
	return false
}

func (lv *Level) requestUpdate(x, y, z int32, records []BlockRecord) <-chan []BlockRecord {
	done := make(chan []BlockRecord, 1)
	lv.UpdateRequest <- updateRequest{
		x:       x,
		y:       y,
		z:       z,
		records: records,
		done:    done,
	}
	return done
}

func (lv *Level) scheduleUpdate(x, y, z int32, records []BlockRecord, delay time.Duration) {
	time.AfterFunc(delay, func() {
		lv.requestUpdate(x, y, z, records)
		// TODO: broadcast update after request
	})
}

func (lv *Level) updateBlock(x, y, z int32, updated *map[[3]int32]struct{}) []BlockRecord {
	var record []BlockRecord
	(*updated)[[3]int32{x, y, z}] = struct{}{}
	block := lv.Get(x, y, z)

	if handler, ok := blockHandlerMap[block.ID]; ok {
		record = append(record, handler(x, y, z, block, lv)...)
	}

	if _, ok := (*updated)[[3]int32{x + 1, y, z}]; !ok && NeedUpdate(lv.GetBlock(x+1, y, z)) {
		record = append(record, lv.updateBlock(x+1, y, z, updated)...)
	}
	if _, ok := (*updated)[[3]int32{x - 1, y, z}]; !ok && NeedUpdate(lv.GetBlock(x-1, y, z)) {
		record = append(record, lv.updateBlock(x-1, y, z, updated)...)
	}
	if _, ok := (*updated)[[3]int32{x, y + 1, z}]; !ok && y < 127 && NeedUpdate(lv.GetBlock(x, y+1, z)) {
		record = append(record, lv.updateBlock(x, y+1, z, updated)...)
	}
	if _, ok := (*updated)[[3]int32{x, y - 1, z}]; !ok && y > 0 && NeedUpdate(lv.GetBlock(x, y-1, z)) {
		record = append(record, lv.updateBlock(x, y-1, z, updated)...)
	}
	if _, ok := (*updated)[[3]int32{x, y, z + 1}]; !ok && NeedUpdate(lv.GetBlock(x, y, z+1)) {
		record = append(record, lv.updateBlock(x, y, z+1, updated)...)
	}
	if _, ok := (*updated)[[3]int32{x, y, z - 1}]; !ok && NeedUpdate(lv.GetBlock(x, y, z-1)) {
		record = append(record, lv.updateBlock(x, y, z-1, updated)...)
	}
	return record
}

// ChunkExists returns if the chunk is loaded on the given chunk coordinates.
func (lv *Level) ChunkExists(cx, cz int32) bool {
	lv.ChunkMutex.Lock()
	_, ok := lv.ChunkMap[[2]int32{cx, cz}]
	lv.ChunkMutex.Unlock()
	return ok
}

// GetChunk returns *Chunk from ChunkMap with given chunk coordinates.
// If the chunk is not loaded, this will try to load a chunk from Provider.
//
// If Provider fails to load the chunk, this function will return nil.
func (lv *Level) GetChunk(cx, cz int32) *Chunk {
	lv.ChunkMutex.Lock()
	defer lv.ChunkMutex.Unlock()
	var err error
	if c, ok := lv.ChunkMap[[2]int32{cx, cz}]; ok {
		return c
	} else if path, ok := lv.Loadable(cx, cz); ok {
		if path == "" {
			goto fallback
		}
		var c *Chunk
		c, err = lv.LoadChunk(cx, cz, path)
		if err != nil {
			goto fallback
		}
		lv.SetChunk(cx, cz, c)
		return c
	}
	return nil
fallback:
	if err != nil {
		log.Println("Error while loading chunk:", err)
	} else {
		log.Println("An error occurred while loading chunk.")
	}
	log.Println("Using empty chunk anyway.")
	c := new(Chunk)
	*c = FallbackChunk
	lv.SetChunk(cx, cz, c)
	return c
}

// SetChunk sets given chunk to chunk map.
// Callers should lock ChunkMutex before call.
func (lv *Level) SetChunk(cx, cz int32, c *Chunk) {
	// lv.ChunkMutex.Lock()
	// defer lv.ChunkMutex.Unlock()
	if _, ok := lv.ChunkMap[[2]int32{cx, cz}]; ok {
		panic("Tried to overwrite existing chunk!")
	}
	lv.ChunkMap[[2]int32{cx, cz}] = c
}

// CreateChunk generates chunk asynchronously.
func (lv *Level) CreateChunk(cx, cz int32) <-chan struct{} {
	done := make(chan struct{}, 1)
	go func(done chan<- struct{}) {
		lv.genTask <- genRequest{
			cx:   cx,
			cz:   cz,
			done: done,
		}
	}(done)
	return done
}

// UnloadChunk unloads chunk from memory.
// If save is given true, this will save the chunk before unload.
//
// Callers should lock ChunkMutex before call.
func (lv *Level) UnloadChunk(cx, cz int32, save bool) error {
	if c, ok := lv.ChunkMap[[2]int32{cx, cz}]; ok {
		delete(lv.ChunkMap, [2]int32{cx, cz})
		if save {
			return lv.WriteChunk(cx, cz, c)
		}
		return nil
	}
	return fmt.Errorf("Chunk %d:%d is not loaded", cx, cz)
}

// Clean unloads all 'unused' chunks from memory.
func (lv *Level) Clean() (cnt int) {
	lv.ChunkMutex.Lock()
	defer lv.ChunkMutex.Unlock()
	cnt = len(lv.CleanQueue)
	for k := range lv.CleanQueue {
		lv.UnloadChunk(k[0], k[1], true)
	}
	return
}

// Save saves all loaded chunks on memory.
func (lv *Level) Save() {
	lv.ChunkMutex.Lock()
	defer lv.ChunkMutex.Unlock()
	if err := lv.SaveAll(lv.ChunkMap); err != nil {
		log.Println("Error while saving level:", err)
	}
}

// GetBlock returns block ID on given coordinates.
func (lv *Level) GetBlock(x, y, z int32) byte {
	c := lv.GetChunk(x>>4, z>>4)
	if c == nil {
		return 0
	}
	c.Mutex().RLock()
	defer c.Mutex().RUnlock()
	return c.GetBlock(byte(x&0xf), byte(y), byte(z&0xf))
}

// SetBlock sets block ID on given coordinates.
func (lv *Level) SetBlock(x, y, z int32, b byte) {
	c := lv.GetChunk(x>>4, z>>4)
	if c == nil {
		return
	}
	c.Mutex().Lock()
	defer c.Mutex().Unlock()
	c.SetBlock(byte(x&0xf), byte(y), byte(z&0xf), b)
}

// GetBlockMeta returns block meta on given coordinates.
func (lv *Level) GetBlockMeta(x, y, z int32) byte {
	c := lv.GetChunk(x>>4, z>>4)
	if c == nil {
		return 0
	}
	c.Mutex().RLock()
	defer c.Mutex().RUnlock()
	return c.GetBlockMeta(byte(x&0xf), byte(y), byte(z&0xf))
}

// SetBlockMeta sets block meta on given coordinates.
func (lv *Level) SetBlockMeta(x, y, z int32, b byte) {
	c := lv.GetChunk(x>>4, z>>4)
	if c == nil {
		return
	}
	c.Mutex().Lock()
	defer c.Mutex().Unlock()
	c.SetBlockMeta(byte(x&0xf), byte(y), byte(z&0xf), b)
}

// Get returns Block struct on given coordinates.
// The struct will contain block ID/meta.
func (lv *Level) Get(x, y, z int32) Block {
	c := lv.GetChunk(x>>4, z>>4)
	if c == nil {
		return Block{}
	}
	c.Mutex().Lock()
	defer c.Mutex().Unlock()
	return Block{
		ID:   c.GetBlock(byte(x&0xf), byte(y), byte(z&0xf)),
		Meta: c.GetBlockMeta(byte(x&0xf), byte(y), byte(z&0xf)),
	}
}

// Set sets block to given Block struct on given coordinates.
func (lv *Level) Set(x, y, z int32, block Block) {
	c := lv.GetChunk(x>>4, z>>4)
	if c == nil {
		return
	}
	c.Mutex().Lock()
	defer c.Mutex().Unlock()
	c.SetBlock(byte(x&0xf), byte(y), byte(z&0xf), block.ID)
	c.SetBlockMeta(byte(x&0xf), byte(y), byte(z&0xf), block.Meta)
}

// SetRecord executes level.Set and creates new BlockRecord for UpdateBlockPacket.
func (lv *Level) SetRecord(x, y, z int32, block Block) BlockRecord {
	lv.Set(x, y, z, block)
	return BlockRecord{
		X:     uint32(x),
		Y:     byte(y),
		Z:     uint32(z),
		Block: Block{ID: block.ID},
		Flags: UpdateAllPriority,
	}
}
