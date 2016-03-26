package highmc

import (
	"bytes"
	"log"
	"net"
)

var handlers map[byte]Protocol
var dataPacketHandlers map[byte]Protocol

// AddressTemplate ...
var AddressTemplate = []*net.UDPAddr{
	{
		IP:   []byte{127, 0, 0, 1},
		Port: 0,
	},
	{
		IP:   []byte{0, 0, 0, 0},
		Port: 0,
	},
	{
		IP:   []byte{0, 0, 0, 0},
		Port: 0,
	},
	{
		IP:   []byte{0, 0, 0, 0},
		Port: 0,
	},
	{
		IP:   []byte{0, 0, 0, 0},
		Port: 0,
	},
	{
		IP:   []byte{0, 0, 0, 0},
		Port: 0,
	},
	{
		IP:   []byte{0, 0, 0, 0},
		Port: 0,
	},
	{
		IP:   []byte{0, 0, 0, 0},
		Port: 0,
	},
	{
		IP:   []byte{0, 0, 0, 0},
		Port: 0,
	},
	{
		IP:   []byte{0, 0, 0, 0},
		Port: 0,
	},
}

// InitProtocol registers raknet packet handlers
func InitProtocol() {
	handlers = map[byte]Protocol{
		0x05: new(openConnectionRequest1),
		0x06: new(openConnectionReply1),
		0x07: new(openConnectionRequest2),
		0x08: new(openConnectionReply2),
		0x80: new(dataPacket),
		0xa0: new(nack),
		0xc0: new(ack),
	}
	dataPacketHandlers = map[byte]Protocol{
		0x00: new(ping),
		0x03: new(pong),
		0x09: new(clientConnect),
		0x10: new(serverHandshake),
		0x13: new(clientHandshake),
		0x15: new(clientDisconnect),
	}
}

// GetHandler returns packet handler with given packet ID.
func GetHandler(pid byte) (proto Protocol) {
	if pid >= 0x80 && pid < 0x90 {
		pid = 0x80
	}
	if v, ok := handlers[pid]; ok {
		return v
	}
	return
}

// GetDataHandler returns datapacket handler with given packet ID.
func GetDataHandler(pid byte) (proto Protocol) {
	if v, ok := dataPacketHandlers[pid]; ok {
		return v
	}
	return
}

func checkFields(m map[string]interface{}, Fields ...string) bool {
	if len(m) != len(Fields) {
		return false
	}
	for _, f := range Fields {
		if _, ok := m[f]; !ok {
			return false
		}
	}
	return true
}

// Fields is shorthand for map[string]interface{}.
type Fields map[string]interface{}

type missingFieldError struct{}

func (e *missingFieldError) Error() string {
	return "Missing required packet fields"
}

// Protocol is a handler interface for Raknet packets.
type Protocol interface {
	Read(*bytes.Buffer) Fields // NOTE: remove first byte(pid) before Read().
	Handle(Fields, *Session)
	Write(Fields) *bytes.Buffer // NOTE: Write() should put pid before encoding with bytes.NewBuffer([]byte{), and should put target session address.
}

type openConnectionRequest1 struct{}

func (p *openConnectionRequest1) Read(buf *bytes.Buffer) (f Fields) {
	f = make(Fields)
	buf.Next(16) // Magic
	f["protocol"] = ReadByte(buf)
	f["mtuSize"] = 18 + buf.Len()
	return
}

func (p *openConnectionRequest1) Handle(f Fields, session *Session) {
	if session.Status > 1 {
		return
	}
	log.Println("Handling OCR1: protocol", f["protocol"], f)
	buf := new(openConnectionReply1).Write(Fields{
		"mtuSize":  f["mtuSize"],
		"serverID": serverID,
	})
	session.Status = 1
	session.send(buf)
}

func (p *openConnectionRequest1) Write(f Fields) (buf *bytes.Buffer) {
	if !checkFields(f, "protocol", "mtuSize") {
		return
	}
	buf = bytes.NewBuffer([]byte{0x05})
	buf.Write([]byte(RaknetMagic))
	WriteByte(buf, f["protocol"].(byte))
	buf.Write(make([]byte, f["mtuSize"].(int)-18))
	return
}

type openConnectionReply1 struct{}

func (p *openConnectionReply1) Read(buf *bytes.Buffer) (f Fields) {
	f = make(Fields)
	buf.Next(16)
	f["serverID"] = ReadLong(buf)
	buf.Next(1)
	f["mtuSize"] = ReadShort(buf)
	return
}

func (p *openConnectionReply1) Handle(f Fields, session *Session) {}

func (p *openConnectionReply1) Write(f Fields) (buf *bytes.Buffer) {
	if !checkFields(f, "serverID", "mtuSize") {
		return
	}
	buf = bytes.NewBuffer([]byte{0x06})
	buf.Write([]byte(RaknetMagic))
	WriteLong(buf, f["serverID"].(uint64))
	WriteByte(buf, 0)
	WriteShort(buf, uint16(f["mtuSize"].(int)))
	return
}

type openConnectionRequest2 struct{}

func (p *openConnectionRequest2) Read(buf *bytes.Buffer) (f Fields) {
	f = make(Fields)
	buf.Next(16)
	f["serverAddress"] = ReadAddress(buf)
	f["mtuSize"] = ReadShort(buf)
	f["clientID"] = ReadLong(buf)
	return
}

func (p *openConnectionRequest2) Handle(f Fields, session *Session) {
	if session.Status != 1 {
		return
	}
	log.Println("Handling OCR2: clientID", f["clientID"])
	session.ID = f["clientID"].(uint64)
	session.mtuSize = f["mtuSize"].(uint16)
	buf := new(openConnectionReply2).Write(Fields{
		"serverID":      serverID,
		"clientAddress": session.Address,
		"mtuSize":       session.mtuSize,
	})
	session.Status = 2
	session.send(buf)
}

func (p *openConnectionRequest2) Write(f Fields) (buf *bytes.Buffer) {
	if !checkFields(f, "serverAddress", "mtuSize", "clientID") {
		return
	}
	buf = bytes.NewBuffer([]byte{0x07})
	buf.Write([]byte(RaknetMagic))
	WriteAddress(buf, f["serverAddress"].(*net.UDPAddr))
	WriteShort(buf, f["mtuSize"].(uint16))
	WriteLong(buf, f["clientID"].(uint64))
	return
}

type openConnectionReply2 struct{}

func (p *openConnectionReply2) Read(buf *bytes.Buffer) (f Fields) {
	f = make(Fields)
	buf.Next(16)
	f["serverID"] = ReadLong(buf)
	f["clientAddress"] = ReadAddress(buf)
	f["mtuSize"] = ReadShort(buf)
	return
}

func (p *openConnectionReply2) Handle(f Fields, session *Session) {}

func (p *openConnectionReply2) Write(f Fields) (buf *bytes.Buffer) {
	if !checkFields(f, "serverID", "clientAddress", "mtuSize") {
		return
	}
	buf = bytes.NewBuffer([]byte{0x08})
	buf.Write([]byte(RaknetMagic))
	WriteLong(buf, f["serverID"].(uint64))
	WriteAddress(buf, f["clientAddress"].(*net.UDPAddr))
	WriteShort(buf, f["mtuSize"].(uint16))
	WriteByte(buf, 0)
	return
}

type dataPacket struct{}

func (p *dataPacket) Read(buf *bytes.Buffer) (f Fields) {
	f = make(Fields)
	dp := new(DataPacket)
	dp.Buffer = buf
	/*
		log.Println("======= DataPacket dump =======")
		log.Println(hex.Dump(dp.Payload))
	*/
	dp.Decode()
	f["seqNumber"] = dp.SeqNumber
	f["packets"] = dp.Packets
	return
}

func (p *dataPacket) Handle(f Fields, session *Session) {
	seqNumber := f["seqNumber"].(uint32)
	packets := f["packets"].([]*EncapsulatedPacket)
	/*
		log.Println("SeqNumber:", seqNumber, "should be", session.windowBorder[0], "<= n <", session.windowBorder[1])
		log.Println("Packets:", len(packets))
		for i, pk := range packets {
			log.Println("Encapsulated #" + strconv.Itoa(i))
			log.Println(hex.Dump(func() []byte {
				b, _ := pk.Bytes()
				return b.Payload
			}()))
			log.Println(hex.Dump(pk.Payload) + "(real data)")
		}
		log.Println("===== DataPacket dump end =====")
	*/
	if seqNumber < session.windowBorder[0] || seqNumber >= session.windowBorder[1] {
		return
	}
	session.packetWindow[seqNumber] = true
	session.ackQueue[seqNumber] = true
	diff := seqNumber - session.lastSeq
	if diff != 1 {
		for i := session.lastSeq + 1; i < seqNumber; i++ {
			if _, ok := session.packetWindow[i]; !ok {
				// log.Println("Seqnumber", i, "is missing. Adding to NACK queue.")
				session.nackQueue[i] = true
			}
		}
	}
	if diff >= 1 {
		session.lastSeq = seqNumber
		session.windowBorder[0] += diff
		session.windowBorder[1] += diff
		for _, pk := range packets {
			session.preEncapsulated(pk)
		}
	}
	return
}

func (p *dataPacket) Write(f Fields) (buf *bytes.Buffer) {
	if !checkFields(f, "seqNumber", "packets") {
		return
	}
	dp := new(DataPacket)
	dp.SeqNumber = f["seqNumber"].(uint32)
	dp.Packets = f["packets"].([]*EncapsulatedPacket)
	dp.Encode()
	buf = dp.Buffer
	return
}

type ack struct{}

func (p *ack) Read(buf *bytes.Buffer) (f Fields) {
	f = make(Fields)
	f["seqs"] = DecodeAck(buf)
	return
}

func (p *ack) Handle(f Fields, session *Session) {
	session.recoveryLock.Lock()
	defer session.recoveryLock.Unlock()
	for _, seq := range f["seqs"].([]uint32) {
		if _, ok := session.recovery[seq]; ok {
			delete(session.recovery, seq)
		}
	}
}

func (p *ack) Write(f Fields) (buf *bytes.Buffer) {
	// Unused, should be directly sent on session.
	return
}

type nack struct{}

func (p *nack) Read(buf *bytes.Buffer) (f Fields) {
	f = make(Fields)
	f["seqs"] = DecodeAck(buf)
	return
}

func (p *nack) Handle(f Fields, session *Session) {
	for _, seq := range f["seqs"].([]uint32) {
		if _, ok := session.nackQueue[seq]; ok {
			delete(session.nackQueue, seq)
		}
	}
}

func (p *nack) Write(f Fields) (buf *bytes.Buffer) {
	// Unused, should be directly sent on session.
	return
}

type clientConnect struct{}

func (p *clientConnect) Read(buf *bytes.Buffer) (f Fields) {
	f = make(Fields)
	f["clientID"] = ReadLong(buf)
	f["sendPing"] = ReadLong(buf)
	f["useSecurity"] = func() bool {
		var b byte
		b = ReadByte(buf)
		return b > 0
	}()
	return
}

func (p *clientConnect) Handle(f Fields, session *Session) {
	if session.Status != 2 {
		return
	}
	buf := new(serverHandshake).Write(Fields{
		"address":         session.Address,
		"systemAddresses": AddressTemplate,
		"sendPing":        f["sendPing"],
		"sendPong":        f["sendPing"].(uint64) + 1000,
	})
	session.sendEncapsulatedDirect(&EncapsulatedPacket{Buffer: buf})
	return
}

func (p *clientConnect) Write(f Fields) (buf *bytes.Buffer) {
	if !checkFields(f, "clientID", "sendPing", "useSecurity") {
		return
	}
	buf = bytes.NewBuffer([]byte{0x09})
	WriteLong(buf, f["clientID"].(uint64))
	WriteLong(buf, f["sendPing"].(uint64))
	WriteByte(buf, func() byte {
		if f["useSecurity"].(bool) {
			return 1
		}
		return 0
	}())
	return
}

type clientDisconnect struct{}

func (p *clientDisconnect) Read(buf *bytes.Buffer) (f Fields) {
	return
}

func (p *clientDisconnect) Handle(f Fields, session *Session) {
	session.Close("client disconnect")
}

func (p *clientDisconnect) Write(f Fields) (buf *bytes.Buffer) {
	return
}

type clientHandshake struct{}

func (p *clientHandshake) Read(buf *bytes.Buffer) (f Fields) {
	f = make(Fields)
	f["address"] = ReadAddress(buf)
	addrs := make([]*net.UDPAddr, 10)
	for i := 0; i < 10; i++ {
		addrs[i] = ReadAddress(buf)
	}
	f["systemAddresses"] = addrs
	f["sendPing"] = ReadLong(buf)
	f["sendPong"] = ReadLong(buf)
	return
}

func (p *clientHandshake) Handle(f Fields, session *Session) {
	log.Println("Client connected successfully!")
	session.Status = 3
	session.connComplete()
	return
}

func (p *clientHandshake) Write(f Fields) (buf *bytes.Buffer) {
	if !checkFields(f, "address", "systemAddresses", "sendPing", "sendPong") {
		return
	}
	buf = bytes.NewBuffer([]byte{0x13})
	WriteAddress(buf, f["address"].(*net.UDPAddr))
	for _, addr := range f["systemAddresses"].([]*net.UDPAddr) {
		WriteAddress(buf, addr)
	}
	WriteLong(buf, f["sendPing"].(uint64))
	WriteLong(buf, f["sendPong"].(uint64))
	return
}

type serverHandshake struct{}

func (p *serverHandshake) Read(buf *bytes.Buffer) (f Fields) {
	f = make(Fields)
	f["address"] = ReadAddress(buf)
	buf.Next(1) // Unknown
	addrs := make([]*net.UDPAddr, 10)
	for i := 0; i < 10; i++ {
		addrs[0] = ReadAddress(buf)
	}
	f["systemAddresses"] = addrs
	f["sendPing"] = ReadLong(buf)
	f["sendPong"] = ReadLong(buf)
	return
}

func (p *serverHandshake) Handle(f Fields, session *Session) {
	return
}

func (p *serverHandshake) Write(f Fields) (buf *bytes.Buffer) {
	if !checkFields(f, "address", "systemAddresses", "sendPing", "sendPong") {
		return
	}
	buf = bytes.NewBuffer([]byte{0x10})
	WriteAddress(buf, f["address"].(*net.UDPAddr))
	WriteByte(buf, 0) // Unknown
	for _, addr := range f["systemAddresses"].([]*net.UDPAddr) {
		WriteAddress(buf, addr)
	}
	WriteLong(buf, f["sendPing"].(uint64))
	WriteLong(buf, f["sendPong"].(uint64))
	return
}

type ping struct{}

func (p *ping) Read(buf *bytes.Buffer) (f Fields) {
	f = make(Fields)
	f["pingID"] = ReadLong(buf)
	return
}

func (p *ping) Handle(f Fields, session *Session) {
	buf := new(pong).Write(Fields{
		"pingID": f["pingID"],
	})
	session.sendEncapsulatedDirect(&EncapsulatedPacket{Buffer: buf})
	return
}

func (p *ping) Write(f Fields) (buf *bytes.Buffer) {
	if !checkFields(f, "pingID") {
		return
	}
	buf = bytes.NewBuffer([]byte{0x00})
	WriteLong(buf, f["pingID"].(uint64))
	return
}

type pong struct{}

func (p *pong) Read(buf *bytes.Buffer) (f Fields) {
	f = make(Fields)
	f["pingID"] = ReadLong(buf)
	return
}

func (p *pong) Handle(f Fields, session *Session) {
	if session.pingTries > 0 {
		session.timeout.Reset(timeout)
		session.pingTries = 0
	}
	return
}

func (p *pong) Write(f Fields) (buf *bytes.Buffer) {
	if !checkFields(f, "pingID") {
		return
	}
	buf = bytes.NewBuffer([]byte{0x03})
	WriteLong(buf, f["pingID"].(uint64))
	return
}
