package highmc

import (
	"bytes"
	"log"
	"math/rand"
	"net"
	"runtime/debug"
	"sync/atomic"
	"time"
)

const windowSize = 2048

// MaxPingTries defines max retry count on ping timeout.
// If ping timeouts MaxPingTries + 1 times, session will be closed.
const MaxPingTries uint64 = 3

// RecoveryTimeout defines how long packets can live on recoery queue.
// Once the packet is sent, the packet will be on recoery queue in RecoveryTimeout duration.
const RecoveryTimeout = time.Second * 8

// SessionLock is a explicit locker for Sessions map.
var timeout = time.Millisecond * 2000

type ackUpdate struct {
	got  bool // true: got ACK/NACK, false: remove ACK/NACK queue
	nack bool // true: NACK, false: ACK
	seqs []uint32
}

type session struct {
	Status           byte
	ReceivedChan     chan Packet // Packet from router
	SendChan         chan Packet // Send request to router
	EncapsulatedChan chan *EncapsulatedPacket
	AckChan          chan ackUpdate

	Player *player
	Server *Server

	ID                 uint64
	Address            *net.UDPAddr
	updateTicker       *time.Ticker
	windowUpdateTicker *time.Ticker
	timeout            *time.Timer
	mtuSize            uint32

	ackQueue  map[uint32]struct{}
	nackQueue map[uint32]struct{}
	recovery  map[uint32]*DataPacket

	packetWindow   map[uint32]bool
	windowBorder   [2]uint32 // Window range: [windowBorder[0], windowBorder[1])
	reliableWindow map[uint32]*EncapsulatedPacket
	reliableBorder [2]uint32 // Window range: [windowBorder[0], windowBorder[1])

	seqNumber    uint32 // Send
	lastSeq      uint32 // Recv
	lastMsgIndex uint32
	splitID      uint16
	splitTable   map[uint16]map[uint32][]byte
	messageIndex uint32
	channelIndex [8]uint32

	playerAdder   func(*net.UDPAddr) chan<- *bytes.Buffer
	playerRemover func(*net.UDPAddr) error
	pingTries     uint64
	closed        chan struct{}
}

// NewSession returns new session instance.
func NewSession(address *net.UDPAddr) *session {
	s := new(session)
	s.Address = address

	s.ReceivedChan = make(chan Packet, chanBufsize)
	s.EncapsulatedChan = make(chan *EncapsulatedPacket, chanBufsize)
	s.AckChan = make(chan ackUpdate, chanBufsize)
	s.closed = make(chan struct{})

	s.updateTicker = time.NewTicker(time.Millisecond * 100)
	s.windowUpdateTicker = time.NewTicker(time.Millisecond * 100)
	s.timeout = time.NewTimer(time.Millisecond * 1500)

	s.ackQueue = make(map[uint32]struct{})
	s.nackQueue = make(map[uint32]struct{})
	s.recovery = make(map[uint32]*DataPacket)

	s.seqNumber = 1<<32 - 1
	s.packetWindow = make(map[uint32]bool)
	s.reliableWindow = make(map[uint32]*EncapsulatedPacket)

	s.splitTable = make(map[uint16]map[uint32][]byte)

	s.windowBorder = [2]uint32{0, windowSize}
	s.reliableBorder = [2]uint32{0, windowSize}

	s.lastSeq = ^uint32(0)
	s.lastMsgIndex = ^uint32(0)
	return s
}

func (s *session) work() {
	for {
		select { // Workaround for first-class priority close signal
		case <-s.closed:
			s.updateTicker.Stop()
			s.timeout.Stop()
			return
		default:
		}
		select {
		case <-s.closed:
			s.updateTicker.Stop()
			s.timeout.Stop()
			return
		case pk := <-s.ReceivedChan:
			s.handlePacket(pk)
		case <-s.timeout.C:
			if s.Status < 3 || s.pingTries >= MaxPingTries {
				s.Close("timeout")
				break
			}
			p := &Ping{PingID: uint64(rand.Uint32())<<32 | uint64(rand.Uint32())}
			buf := Pool.NewBuffer(nil)
			p.Write(buf)
			s.sendEncapsulatedDirect(&EncapsulatedPacket{Buffer: buf})
			s.pingTries++
			s.timeout.Reset(timeout)
		case <-s.windowUpdateTicker.C:
			s.windowUpdate()
		}
	}
}

func (s *session) sendAsync() {
	for {
		select { // Workaround for first-class priority close signal
		case <-s.closed:
			s.updateTicker.Stop()
			s.timeout.Stop()
		default:
		}
		select {
		case <-s.closed:
			return
		case ep := <-s.EncapsulatedChan:
			dp := new(DataPacket)
			dp.Head = 0x80
			dp.SeqNumber = atomic.AddUint32(&s.seqNumber, 1)
			dp.Packets = []*EncapsulatedPacket{ep}
			dp.Encode()
			s.send(dp.Buffer)
			dp.SendTime = time.Now()
			s.recovery[dp.SeqNumber] = dp
		case u := <-s.AckChan:
			s.handleAckUpdate(u)
		case <-s.updateTicker.C:
			s.update()
		}
	}
}

func (s *session) update() {
	if len(s.ackQueue) > 0 {
		acks := make([]uint32, len(s.ackQueue))
		i := 0
		for k := range s.ackQueue {
			acks[i] = k
			i++
		}
		buf := EncodeAck(acks)
		b := Pool.NewBuffer([]byte{0xc0})
		Write(b, buf.Bytes())
		s.send(b)
		s.ackQueue = make(map[uint32]struct{})
	}
	if len(s.nackQueue) > 0 {
		nacks := make([]uint32, len(s.nackQueue))
		i := 0
		for k := range s.nackQueue {
			nacks[i] = k
			i++
		}
		buf := EncodeAck(nacks)
		b := Pool.NewBuffer([]byte{0xa0})
		Write(b, buf.Bytes())
		s.send(b)
		s.nackQueue = make(map[uint32]struct{})
	}
	for seq, pk := range s.recovery {
		if pk.SendTime.Add(RecoveryTimeout).Before(time.Now()) {
			s.send(pk.Buffer)
			Pool.Recycle(pk.Buffer)
			delete(s.recovery, seq)
		} else {
			break
		}
	}
}

func (s *session) windowUpdate() {
	for seq := range s.packetWindow {
		if seq < atomic.LoadUint32(&s.windowBorder[0]) {
			delete(s.packetWindow, seq)
		} else {
			break
		}
	}
}

func (s *session) handleAckUpdate(u ackUpdate) {
	if u.got {
		if u.nack {
			for _, seq := range u.seqs {
				if dp, ok := s.recovery[seq]; ok {
					s.send(dp.Buffer)
				}
			}
		} else {
			for _, seq := range u.seqs {
				if _, ok := s.recovery[seq]; ok {
					delete(s.recovery, seq)
				}
			}
		}
	} else {
		if u.nack {
			for _, seq := range u.seqs {
				s.nackQueue[seq] = struct{}{}
			}
		} else {
			for _, seq := range u.seqs {
				s.ackQueue[seq] = struct{}{}
			}
		}
	}
}

func (s *session) handlePacket(pk Packet) {
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		if _, ok := r.(Overflow); ok {
			log.Println("Recovering panic:", r)
			Dump(pk.Buffer)
			debug.PrintStack()
		}
	}()
	head := ReadByte(pk.Buffer)
	if head != 0xa0 && head != 0xc0 {
		s.timeout.Reset(func() time.Duration {
			if s.Status != 3 {
				return time.Millisecond * 1500
			}
			return timeout
		}())
	}
	if handler := GetRaknetPacket(head); handler != nil {
		handler.Read(pk.Buffer)
		handler.Handle(s)
		Pool.Recycle(pk.Buffer)
	}
}

func (s *session) preEncapsulated(ep *EncapsulatedPacket) {
	if ep.Reliability >= 2 && ep.Reliability != 5 { // MessageIndex exists
		if ep.MessageIndex < s.reliableBorder[0] || ep.MessageIndex >= s.reliableBorder[1] { // Outside of window
			//log.Println("MessageIndex drop:", ep.MessageIndex, "should be", s.reliableBorder[0], "<= n <", s.reliableBorder[1])
			return
		}
		if ep.MessageIndex-s.lastMsgIndex == 1 {
			s.lastMsgIndex++
			s.reliableBorder[0]++
			s.reliableBorder[1]++
			s.handleEncapsulated(ep)
			if len(s.reliableWindow) > 0 {
				for _, i := range GetSortedKeys(s.reliableWindow) {
					if uint32(i)-s.lastMsgIndex != 1 {
						break
					}
					s.lastMsgIndex++
					s.reliableBorder[0]++
					s.reliableBorder[1]++
					s.handleEncapsulated(s.reliableWindow[uint32(i)])
					delete(s.reliableWindow, uint32(i))
				}
			}
		} else {
			s.reliableWindow[ep.MessageIndex] = ep
		}
	} else {
		s.handleEncapsulated(ep)
	}
}

func (s *session) joinSplits(ep *EncapsulatedPacket) {
	if s.Status < 3 {
		return
	}
	tab, ok := s.splitTable[ep.SplitID]
	if !ok {
		s.splitTable[ep.SplitID] = make(map[uint32][]byte)
		tab = s.splitTable[ep.SplitID]
	}
	if _, ok := tab[ep.SplitIndex]; !ok {
		tab[ep.SplitIndex] = ep.Buffer.Bytes()
	}
	if len(tab) == int(ep.SplitCount) {
		sep := new(EncapsulatedPacket)
		sep.Buffer = Pool.NewBuffer(nil)
		for i := uint32(0); i < ep.SplitCount; i++ {
			sep.Write(tab[i])
		}
		delete(s.splitTable, ep.SplitID)
		s.handleEncapsulated(sep)
	}
}

func (s *session) handleEncapsulated(ep *EncapsulatedPacket) {
	if ep.HasSplit {
		if s.Status > 2 {
			s.joinSplits(ep)
		} else {
			log.Println("Warning: Got split packet in connecting state")
		}
		return
	}
	head := ReadByte(ep.Buffer)

	if s.Status > 2 && head == 0x8e {
		s.Player.HandlePacket(ep.Buffer)
		return
	}

	if handler := GetDataPacket(head); handler != nil {
		handler.Read(ep.Buffer)
		handler.Handle(s)
	}
	Pool.Recycle(ep.Buffer)
}

func (s *session) connComplete() {
	if s.Status != 3 {
		return
	}
	s.Player = NewPlayer(s)
}

// SendEncapsulated processes EncapsulatedPacket informations before sending.
func (s *session) SendEncapsulated(ep *EncapsulatedPacket) {
	if ep.Reliability >= 2 && ep.Reliability != 5 {
		ep.MessageIndex = s.messageIndex
		s.messageIndex++
	}
	if ep.Reliability <= 4 && ep.Reliability != 2 {
		ep.OrderIndex = s.channelIndex[ep.OrderChannel]
		s.channelIndex[ep.OrderChannel]++
	}
	if ep.TotalLen()+4 > int(s.mtuSize) { // Need split
		splitID := s.splitID
		s.splitID++
		splitIndex := uint32(0)
		mtu := (atomic.LoadUint32(&s.mtuSize) - 34)
		splitCount := uint32(ep.Len()) / mtu
		if uint32(ep.Len())%mtu != 0 {
			splitCount++
		}
		for ep.Len() > 0 {
			buf := ep.Next(int(mtu))
			sp := new(EncapsulatedPacket)
			sp.SplitID = splitID
			sp.HasSplit = true
			sp.SplitCount = splitCount
			sp.Reliability = ep.Reliability
			sp.SplitIndex = splitIndex
			sp.Buffer = Pool.NewBuffer(buf)
			sp.MessageIndex = s.messageIndex
			s.messageIndex++
			if sp.Reliability == 3 {
				sp.OrderChannel = ep.OrderChannel
				sp.OrderIndex = ep.OrderIndex
			}
			splitIndex++
			s.EncapsulatedChan <- sp
		}
	} else {
		s.EncapsulatedChan <- ep
	}
}

func (s *session) sendEncapsulatedDirect(ep *EncapsulatedPacket) {
	dp := new(DataPacket)
	dp.Head = 0x80
	dp.SeqNumber = atomic.AddUint32(&s.seqNumber, 1)
	dp.Packets = []*EncapsulatedPacket{ep}
	dp.Encode()
	s.send(dp.Buffer)
}

func (s *session) send(pk *bytes.Buffer) {
	s.SendChan <- Packet{pk, s.Address}
}

// Close stops current session.
func (s *session) Close(reason string) {
	select {
	case <-s.closed: // Already closed
		log.Println("Warning: duplicate close attempt")
		return
	default:
	}
	close(s.closed)
	data := &EncapsulatedPacket{Buffer: Pool.NewBuffer([]byte{0x15})}
	s.sendEncapsulatedDirect(data)
	log.Println("Session closed:", reason)
}
