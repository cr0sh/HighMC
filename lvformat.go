package highmc

import (
	"reflect"
	"strings"
)

var levelProviders = map[string]LevelProvider{}

// LevelProvider is a interface for level formats.
type LevelProvider interface {
	Init(string)                      // Level name: usually used for file directories
	Loadable(ChunkPos) (string, bool) // Path: path to file, Ok: true if the chunk is saved on the file
	LoadChunk(ChunkPos, string) (*Chunk, error)
	WriteChunk(ChunkPos, *Chunk) error
	SaveAll(map[ChunkPos]*Chunk) error
}

// RegisterProvider adds level format provider for server.
func RegisterProvider(provider LevelProvider) {
	typsl := strings.Split(reflect.TypeOf(provider).String(), ".")
	name := strings.ToLower(typsl[len(typsl)-1])
	if _, ok := levelProviders[name]; !ok {
		levelProviders[name] = provider
	}
}

// GetProvider finds the provider with given name.
// If it doesn't present, returns nil.
func GetProvider(name string) LevelProvider {
	if pv, ok := levelProviders[name]; ok {
		return pv
	}
	return nil
}
