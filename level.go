package highmc

import (
	"sync"
)

// BlockPos is a type for x-y-z block coordinates.
type BlockPos struct {
	X, Z int32
	Y    byte
}

// LevelReader is a level interface which allows Get* operations.
type LevelReader interface {
	Available(int32, int32) bool
	Get(int32, int32, byte) Block
	GetID(int32, int32, byte) byte
	GetMeta(int32, int32, byte) byte
}

// LevelWriter is a level interface which allows both Get/Set operations.
type LevelWriter interface {
	Set(int32, int32, byte, Block)
	SetID(int32, int32, byte)
	SetMeta(int32, int32, byte)
}

// LevelReadWriter = LevelReader + LevelWriter
type LevelReadWriter interface {
	LevelReader
	LevelWriter
}

// StagedWriter is a wrapper for LevelWriter which buffers every write operations on stage, and batches write when executing Commit().
type StagedWriter struct {
	*Level
	wrap  LevelReadWriter
	stage map[BlockPos]Block
}

// NewStagedWriter returns new StagedWriter object, with given LevelReadWriter wrapped.
func NewStagedWriter(wrap LevelReadWriter) *StagedWriter {
	return &StagedWriter{wrap: wrap, stage: make(map[BlockPos]Block)}
}

// Commit batches all staged write operations and resets the stage.
func (sw *StagedWriter) Commit() {
	// TODO
}

// Set wraps Level.Set method.
func (sw *StagedWriter) Set(x, z int32, y byte, b Block) {
	sw.stage[BlockPos{X: x, Y: y, Z: z}] = b
}

// SetID wraps Level.SetID method.
func (sw *StagedWriter) SetID(x, z int32, y byte, i byte) {
	if st, ok := sw.stage[BlockPos{X: x, Y: y, Z: z}]; ok {
		sw.stage[BlockPos{X: x, Y: y, Z: z}] = Block{
			ID:   i,
			Meta: sw.stage[BlockPos{X: x, Y: y, Z: z}].Meta,
		}
	} else {
		sw.stage[BlockPos{X: x, Y: y, Z: z}] = Block{
			ID:   i,
			Meta: sw.GetMeta(x, z, y),
		}
	}
}

// SetMeta wraps Level.SetMeta method.
func (sw *StagedWriter) SetMeta(x, z int32, y byte, m byte) {
	if st, ok := sw.stage[BlockPos{X: x, Y: y, Z: z}]; ok {
		sw.stage[BlockPos{X: x, Y: y, Z: z}] = Block{
			ID:   sw.stage[BlockPos{X: x, Y: y, Z: z}].ID,
			Meta: m,
		}
	} else {
		sw.stage[BlockPos{X: x, Y: y, Z: z}] = Block{
			ID:   sw.GetID(x, z, y),
			Meta: m,
		}
	}
}

// Level is a struct to manage single MCPE world.
// Accessing level blocks must be done on level callbacks with Level.(RO/RW)(Async/*) func.
//
// If you are writing many blocks to the level, use StagedWriter.
//
// Use *Async func to run callback on level goroutine, asynchronously.
type Level struct {
	LoadedChunks map[ChunkPos]*Chunk

	roChan chan func(LevelReader)
	rwChan chan func(LevelReadWriter)
	mutex  *sync.RWMutex
}

// Init initializes the level.
func (lv *Level) Init() {
	lv.LoadedChunks = make(map[ChunkPos]*Chunk)

	lv.roChan = make(chan func(LevelReader), chanBufsize)
	lv.rwChan = make(chan func(LevelReadWriter), chanBufsize)
	lv.mutex = new(sync.RWMutex)
}

func (lv *Level) process() {
	for {
		select {
		case callback := <-lv.roChan:
			lv.RO(callback)
		case callback := <-lv.rwChan:
			lv.RW(callback)
		}
	}
}

// Available returns whether given coordinate's chunk is loaded or not.
func (lv *Level) Available(x, z int32) bool {
	_, ok := lv.LoadedChunks[ChunkPos{x >> 4, z >> 4}]
	return ok
}

// Get returns Block from level.
func (lv *Level) Get(x, z int32, y byte) Block {
	return Block{}
}

// GetID returns Block ID from level.
func (lv *Level) GetID(x, z int32, y byte) byte {
	return 0
}

// GetMeta returns Block Meta from level.
func (lv *Level) GetMeta(x, z int32, y byte) byte {
	return 0
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

// ROAsync executes RO callback on Level.process goroutine.
func (lv *Level) ROAsync(callback func(LevelReader)) {
	lv.roChan <- callback
}

// RWAsync executes RW callback on Level.process goroutine.
func (lv *Level) RWAsync(callback func(LevelReadWriter)) {
	lv.rwChan <- callback
}
