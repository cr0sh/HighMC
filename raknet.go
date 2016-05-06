package highmc

import (
	"strconv"
	"sync/atomic"
)

const (
	// RaknetMagic is a magic bytes for internal Raknet protocol.
	RaknetMagic = "\x00\xff\xff\x00\xfe\xfe\xfe\xfe\xfd\xfd\xfd\xfd\x12\x34\x56\x78"
	// RaknetProtocol is a internal Raknet protocol version.
	RaknetProtocol = 7
	// MinecraftProtocol is a mojang network protocol version.
	MinecraftProtocol = 60
	// MinecraftVersion is a human readable minecraft version.
	MinecraftVersion = "0.14.2"
)

// ServerName contains human readable server name
var ServerName string

// OnlinePlayers is count of online players
var OnlinePlayers int32

// MaxPlayers is count of maximum available players
var MaxPlayers int32

// GetServerString returns server status message for unconnected pong
func GetServerString() string {
	return "MCPE;" + ServerName + ";" +
		strconv.Itoa(MinecraftProtocol) + ";" +
		MinecraftVersion + ";" +
		strconv.Itoa(int(atomic.LoadInt32(&OnlinePlayers))) + ";" +
		strconv.Itoa(int(atomic.LoadInt32(&MaxPlayers)))
}
