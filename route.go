package highmc

import (
	"bytes"
	"fmt"
	"math/rand"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

var serverID uint64
var blockList = make(map[string]time.Time)
var blockLock = new(sync.Mutex)

// GotBytes is a sum of received packet size.
var GotBytes uint64

// Router handles packets from network, and manages sessions.
type Router struct {
	sessions      []Session
	conn          *net.UDPConn
	sendChan      chan Packet
	playerAdder   func(*net.UDPAddr) chan<- *bytes.Buffer
	playerRemover func(*net.UDPAddr) error
}

// CreateRouter create/opens new raknet router with given port.
func CreateRouter(port uint16) (r *Router, err error) {
	InitProtocol()
	Sessions = make(map[string]*Session)
	r = new(Router)
	serverID = uint64(rand.Int63())
	r.sessions = make([]Session, 0)
	r.sendChan = make(chan Packet, chanBufsize)
	r.conn, err = net.ListenUDP("udp", &net.UDPAddr{Port: int(port)})
	// r.playerAdder = playerAdder
	// r.playerRemover = playerRemover
	return
}

// Start makes router process network I/O operations.
func (r *Router) Start() {
	go r.sendAsync()
	go r.receivePacket()
}

func (r *Router) receivePacket() {
	var recvbuf []byte
	defer r.conn.Close()
	for {
		var n int
		var addr *net.UDPAddr
		var err error
		recvbuf = make([]byte, 1024*1024)
		if n, addr, err = r.conn.ReadFromUDP(recvbuf); err != nil {
			fmt.Println("Error while reading packet:", err)
			continue
		} else if n > 0 {
			atomic.AddUint64(&GotBytes, uint64(n))
			buf := bytes.NewBuffer(recvbuf[0:n])
			pk := Packet{
				Buffer:  buf,
				Address: addr,
			}
			if c, err := buf.ReadByte(); err == nil && c == 0x01 { // Unconnected ping: no need to create session
				pingID := ReadLong(buf)
				buf := new(bytes.Buffer)
				WriteByte(buf, 0x1c)
				WriteLong(buf, pingID)
				WriteLong(buf, serverID)
				buf.Write([]byte(RaknetMagic))
				WriteString(buf, GetServerString())
				pk := Packet{
					Buffer:  buf,
					Address: addr,
				}
				r.sendPacket(pk)
				continue
			}
			buf.UnreadByte()
			func() {
				blockLock.Lock()
				defer blockLock.Unlock()
				if blockList[addr.String()].After(time.Now()) {
					r.conn.WriteToUDP([]byte("\x80\x00\x00\x00\x00\x00\x08\x15"), pk.Address)
				} else {
					delete(blockList, addr.String())
					go func() {
						GetSession(addr, r.sendChan, r.playerAdder, r.playerRemover).ReceivedChan <- pk
					}()
				}
			}()
		}
	}
}

func (r *Router) sendAsync() {
	for pk := range r.sendChan {
		r.sendPacket(pk)
	}
}

func (r *Router) sendPacket(pk Packet) {
	r.conn.WriteToUDP(pk.Bytes(), pk.Address)
}
