package highmc

import (
	"runtime"
	"sync"
)

// BlockPos is a type for x-y-z block coordinates.
type BlockPos struct {
	X, Z int32
	Y    byte
}

// LevelReader is a level interface which allows Get* operations.
type LevelReader interface {
	Available(BlockPos) bool
	Get(BlockPos) Block
	GetID(BlockPos) byte
	GetMeta(BlockPos) byte
}

// LevelWriter is a level interface which allows both Get/Set operations.
type LevelWriter interface {
	Set(BlockPos, Block)
	SetID(BlockPos, byte)
	SetMeta(BlockPos, byte)
	CreateChunk(ChunkPos) *Chunk
}

// LevelReadWriter = LevelReader + LevelWriter
type LevelReadWriter interface {
	LevelReader
	LevelWriter
}

// StagedWriter is a wrapper for LevelReadWriter which buffers every write operations on stage, and batches write when executing Commit().
type StagedWriter struct {
	wrap  LevelReadWriter
	stage map[BlockPos]Block
}

// NewStagedWriter returns new StagedWriter object, with given LevelReadWriter wrapped.
func NewStagedWriter(wrap LevelReadWriter) *StagedWriter {
	return &StagedWriter{wrap: wrap, stage: make(map[BlockPos]Block)}
}

// Commit batches all staged write operations and flushes the stage.
func (sw *StagedWriter) Commit() {
	for pos, block := range sw.stage {
		sw.wrap.Set(pos, block)
	}
	sw.stage = make(map[BlockPos]Block)
}

// Set wraps Level.Set method.
func (sw *StagedWriter) Set(p BlockPos, b Block) {
	sw.stage[p] = b
}

// SetID wraps Level.SetID method.
func (sw *StagedWriter) SetID(p BlockPos, i byte) {
	if st, ok := sw.stage[p]; ok {
		sw.stage[p] = Block{
			ID:   i,
			Meta: st.Meta,
		}
	} else {
		sw.stage[p] = Block{
			ID:   i,
			Meta: sw.wrap.GetMeta(p),
		}
	}
}

// SetMeta wraps Level.SetMeta method.
func (sw *StagedWriter) SetMeta(p BlockPos, m byte) {
	if st, ok := sw.stage[p]; ok {
		sw.stage[p] = Block{
			ID:   st.ID,
			Meta: m,
		}
	} else {
		sw.stage[p] = Block{
			ID:   sw.wrap.GetID(p),
			Meta: m,
		}
	}
}

// CreateChunk wraps Level.CreateChunk method: do not use.
func (sw *StagedWriter) CreateChunk(pos ChunkPos) *Chunk {
	panic("Use of CreateChunk on StagedWriter is unavailable")
}

type chunkRequest struct {
	pos   ChunkPos
	reply chan *Chunk
}

// Level is a struct to manage single MCPE world.
// Accessing level blocks must be done on level callbacks with Level.(RO/RW)(Async/*) func.
//
// If you are writing many blocks to the level, use StagedWriter to buffer write operations.
//
type Level struct {
	LoadedChunks map[ChunkPos]*Chunk

	Name     string
	Server   *Server
	Provider LevelProvider

	roChan       chan func(LevelReader)
	rwChan       chan func(LevelReadWriter)
	chunkRequest chan chunkRequest
	mutex        *sync.RWMutex
}

// Init initializes the level.
func (lv *Level) Init() {
	lv.LoadedChunks = make(map[ChunkPos]*Chunk)
	lv.Provider.Init("default")

	lv.roChan = make(chan func(LevelReader), chanBufsize)
	lv.rwChan = make(chan func(LevelReadWriter), chanBufsize)
	lv.chunkRequest = make(chan chunkRequest, chanBufsize)
	lv.mutex = new(sync.RWMutex)
}

func (lv *Level) process() {
	replyChans := make(map[ChunkPos][]chan<- *Chunk)
	requestChan := make(chan chunkRequest, chanBufsize)
	replyChan := make(chan *Chunk, chanBufsize)
	n := runtime.NumCPU()
	for i := 0; i < n; i++ {
		go lv.chunkWorker(requestChan)
	}
	for {
		select {
		/*
			case callback := <-lv.roChan:
				lv.RO(callback)
			case callback := <-lv.rwChan:
				lv.RW(callback)
		*/
		case req := <-lv.chunkRequest:
			replyChans[req.pos] = append(replyChans[req.pos], req.reply)
			req.reply = replyChan
		case rep := <-replyChan:
			if chs, ok := replyChans[rep.Position]; ok {
				for _, ch := range chs {
					ch <- rep
				}
			} else {
				panic("Reply chunk position is invalid")
			}
		}
	}
}

func (lv *Level) chunkWorker(request chan chunkRequest) {
	for req := range request {
		if dir, ok = lv.Provider.Loadable(req.pos); ok { // file exists
			chunk, err := lv.Provider.LoadChunk(req.pos, dir)
			if err != nil {
				panic("Chunk load error")
			}
			req.reply <- chunk
		} else {
			// Create chunk
			req.reply <- nil // TODO
		}
	}
}

// Available returns whether given block is loaded.
func (lv *Level) Available(pos BlockPos) bool {
	_, ok := lv.LoadedChunks[GetChunkPos(pos)]
	return ok
}

// Lock is a wrapping func for RWMutex.Lock()
func (lv *Level) Lock() {
	lv.mutex.Lock()
}

// Unlock is a wrapping func for RWMutex.Unlock()
func (lv *Level) Unlock() {
	lv.mutex.Unlock()
}

// RLock is a wrapping func for RWMutex.RLock()
func (lv *Level) RLock() {
	lv.mutex.RLock()
}

// RUnlock is a wrapping func for RWMutex.RUnlock()
func (lv *Level) RUnlock() {
	lv.mutex.RUnlock()
}

// Get returns Block from level.
func (lv *Level) Get(p BlockPos) Block {
	return Block{
		ID:   lv.LoadedChunks[GetChunkPos(p)].GetBlock(byte(p.X&0xf), p.Y, byte(p.Z&0xf)),
		Meta: lv.LoadedChunks[GetChunkPos(p)].GetBlockMeta(byte(p.X&0xf), p.Y, byte(p.Z&0xf)),
	}
}

// GetID returns Block ID from level.
func (lv *Level) GetID(p BlockPos) byte {
	return lv.LoadedChunks[GetChunkPos(p)].GetBlock(byte(p.X&0xf), p.Y, byte(p.Z&0xf))
}

// GetMeta returns Block Meta from level.
func (lv *Level) GetMeta(p BlockPos) byte {
	return lv.LoadedChunks[GetChunkPos(p)].GetBlockMeta(byte(p.X&0xf), p.Y, byte(p.Z&0xf))
}

// Set sets block ID/Meta to level.
func (lv *Level) Set(p BlockPos, b Block) {
	lv.LoadedChunks[GetChunkPos(p)].SetBlock(byte(p.X&0xf), p.Y, byte(p.Z&0xf), b.ID)
	lv.LoadedChunks[GetChunkPos(p)].SetBlockMeta(byte(p.X&0xf), p.Y, byte(p.Z&0xf), b.Meta)
}

// SetID sets block ID to level.
func (lv *Level) SetID(p BlockPos, i byte) {
	lv.LoadedChunks[GetChunkPos(p)].SetBlock(byte(p.X&0xf), p.Y, byte(p.Z&0xf), i)
}

// SetMeta sets block Meta to level.
func (lv *Level) SetMeta(p BlockPos, m byte) {
	lv.LoadedChunks[GetChunkPos(p)].SetBlock(byte(p.X&0xf), p.Y, byte(p.Z&0xf), m)
}

// RO executes given level callback in Read-Only mode.
func (lv *Level) RO(callback func(LevelReader)) {
	lv.mutex.RLock()
	defer lv.mutex.RUnlock()
	callback(lv)
}

// RW executes given level callback in Read-Write mode.
func (lv *Level) RW(callback func(LevelReadWriter)) {
	lv.mutex.Lock()
	defer lv.mutex.Unlock()
	callback(lv)
}

/*
// ROAsync executes RO callback on Level.process goroutine.
func (lv *Level) ROAsync(callback func(LevelReader)) {
	lv.roChan <- callback
}

// RWAsync executes RW callback on Level.process goroutine.
func (lv *Level) RWAsync(callback func(LevelReadWriter)) {
	lv.rwChan <- callback
}
*/

// CreateChunk creates the chunk on given ChunkPos.
func (lv *Level) CreateChunk(pos ChunkPos) *Chunk {
	ch := make(chan *Chunk, 1)
	lv.chunkRequest <- chunkRequest{
		pos:   pos,
		reply: ch,
	}
	return <-ch
}
