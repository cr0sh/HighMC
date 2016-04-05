package highmc

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"log"
	"math/rand"
	"net"
	"time"
)

var serverID uint64
var blockList = make(map[string]time.Time)

// Router handles packets from network, and manages sessions.
type Router struct {
	conn        *net.UDPConn
	sendChan    chan Packet
	recvChan    chan Packet
	closeNotify chan *net.UDPAddr
	recvBuf     []byte

	Sessions map[string]*Session
	Owner    *Server
}

// CreateRouter create/opens new raknet router with given port.
func CreateRouter(port uint16) (r *Router, err error) {
	r = new(Router)
	serverID = uint64(rand.Int63())
	r.sendChan = make(chan Packet, chanBufsize)
	r.recvChan = make(chan Packet, chanBufsize)
	r.conn, err = net.ListenUDP("udp", &net.UDPAddr{Port: int(port)})
	r.closeNotify = make(chan *net.UDPAddr, chanBufsize)
	r.Sessions = make(map[string]*Session)
	// r.playerAdder = playerAdder
	// r.playerRemover = playerRemover
	return
}

// GetSession returns session with given identifier if exists, or creates new one.
func (r *Router) GetSession(address *net.UDPAddr, sendChannel chan Packet) *Session {
	if s, ok := r.Sessions[address.String()]; ok {
		return s
	}
	log.Println("New session:", address)
	sess := new(Session)
	sess.Init(address)
	sess.SendChan = sendChannel
	sess.Server = r.Owner
	go sess.sendAsync()
	go sess.work()
	r.Sessions[address.String()] = sess
	return sess
}

// Start makes router process network I/O operations.
func (r *Router) Start() {
	go r.sendAsync()
	go r.receivePacket()
	go r.work()
}

func (r *Router) work() {
	defer r.conn.Close()
	for {
		select {
		case s := <-r.closeNotify:
			delete(r.Sessions, s.String())
			blockList[s.String()] = time.Now().Add(time.Second + time.Millisecond*750)
		case pk := <-r.recvChan:
			if blockList[pk.Address.String()].After(time.Now()) {
				r.conn.WriteToUDP([]byte("\x80\x00\x00\x00\x00\x00\x08\x15"), pk.Address)
			} else {
				delete(blockList, pk.Address.String())
				r.GetSession(pk.Address, r.sendChan).ReceivedChan <- pk
			}
		default:
			r.updateSession()
		}
	}
}

func (r *Router) receivePacket() {
	var n int
	var addr *net.UDPAddr
	var err error
	for {
		r.recvBuf = make([]byte, 1024*1024)
		if n, addr, err = r.conn.ReadFromUDP(r.recvBuf); err != nil {
			log.Println("Error while reading packet:", err)
			continue
		} else if n > 0 {
			buf := bytes.NewBuffer(r.recvBuf[0:n])
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
			r.recvChan <- pk
		}
	}
}

func (r *Router) updateSession() {
	for _, sess := range r.Sessions {
		select {
		case <-sess.closed:
			r.closeNotify <- sess.Address
		default:
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
