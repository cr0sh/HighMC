package highmc

import (
	"bytes"
	"io"
	"log"
	"net"
)

var handlers = map[byte]RaknetPacket{
	0x05: new(OpenConnectionRequest1),
	0x06: new(OpenConnectionReply1),
	0x07: new(OpenConnectionRequest2),
	0x08: new(OpenConnectionReply2),
	0x80: new(GeneralDataPacket),
	0xa0: new(Nack),
	0xc0: new(Ack),
}
var dataPacketHandlers = map[byte]RaknetPacket{
	0x00: new(Ping),
	0x03: new(Pong),
	0x09: new(ClientConnect),
	0x10: new(ServerHandshake),
	0x13: new(ClientHandshake),
	0x15: new(ClientDisconnect),
}

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

// GetRaknetPacket returns raknet packet with given packet ID.
func GetRaknetPacket(pid byte) (proto RaknetPacket) {
	if pid >= 0x80 && pid < 0x90 {
		pid = 0x80
	}
	if v, ok := handlers[pid]; ok {
		return v
	}
	return
}

// GetDataPacket returns datapacket with given packet ID.
func GetDataPacket(pid byte) (proto RaknetPacket) {
	if v, ok := dataPacketHandlers[pid]; ok {
		return v
	}
	return
}

// RaknetPacket is a handler interface for Raknet packets.
type RaknetPacket interface {
	Read(*bytes.Buffer) // NOTE: remove first byte(pid) before Read().
	Handle(*Session)
	Write(*bytes.Buffer) // NOTE: Write() should put pid before encoding with bytes.NewBuffer([]byte{), and should put target session address.
}

// OpenConnectionRequest1 is a packet used in Raknet.
type OpenConnectionRequest1 struct {
	Protocol byte
	MtuSize  int
}

// Read implements RaknetPacket interfaces.
func (pk *OpenConnectionRequest1) Read(buf *bytes.Buffer) {
	buf.Next(16) // Magic
	pk.Protocol = ReadByte(buf)
	pk.MtuSize = 18 + buf.Len()
}

// Handle implements RaknetPacket interfaces.
func (pk *OpenConnectionRequest1) Handle(session *Session) {
	if session.Status > 1 {
		return
	}
	log.Println("Handling OCR1: Protocol", pk.Protocol)
	buf := new(bytes.Buffer)
	p := &OpenConnectionReply1{
		ServerID: serverID,
		MtuSize:  uint16(pk.MtuSize),
	}
	p.Write(buf)
	session.Status = 1
	session.send(buf)
}

// Write implements RaknetPacket interfaces.
func (pk *OpenConnectionRequest1) Write(buf *bytes.Buffer) {
	buf.WriteByte(0x05)
	buf.Write([]byte(RaknetMagic))
	WriteByte(buf, pk.Protocol)
	buf.Write(make([]byte, pk.MtuSize-18))
}

// OpenConnectionReply1 is a packet used in Raknet.
type OpenConnectionReply1 struct {
	ServerID uint64
	MtuSize  uint16
}

// Read implements RaknetPacket interfaces.
func (pk *OpenConnectionReply1) Read(buf *bytes.Buffer) {
	buf.Next(16)
	pk.ServerID = ReadLong(buf)
	buf.Next(1)
	pk.MtuSize = ReadShort(buf)
}

// Handle implements RaknetPacket interfaces.
func (pk *OpenConnectionReply1) Handle(session *Session) {}

// Write implements RaknetPacket interfaces.
func (pk *OpenConnectionReply1) Write(buf *bytes.Buffer) {
	buf.WriteByte(0x06)
	buf.Write([]byte(RaknetMagic))
	WriteLong(buf, pk.ServerID)
	WriteByte(buf, 0)
	WriteShort(buf, uint16(pk.MtuSize))
}

// OpenConnectionRequest2 is a packet used in Raknet.
type OpenConnectionRequest2 struct {
	ServerAddress *net.UDPAddr
	MtuSize       uint16
	ClientID      uint64
}

// Read implements RaknetPacket interfaces.
func (pk *OpenConnectionRequest2) Read(buf *bytes.Buffer) {
	buf.Next(16)
	pk.ServerAddress = ReadAddress(buf)
	pk.MtuSize = ReadShort(buf)
	pk.ClientID = ReadLong(buf)
}

// Handle implements RaknetPacket interfaces.
func (pk *OpenConnectionRequest2) Handle(session *Session) {
	if session.Status != 1 {
		return
	}
	log.Println("Handling OCR2: clientID", pk.ClientID)
	session.ID = pk.ClientID
	session.mtuSize = pk.MtuSize
	buf := new(bytes.Buffer)
	p := &OpenConnectionReply2{
		ServerID:      serverID,
		ClientAddress: session.Address,
		MtuSize:       pk.MtuSize,
	}
	p.Write(buf)
	session.Status = 2
	session.send(buf)
}

// Write implements RaknetPacket interfaces.
func (pk *OpenConnectionRequest2) Write(buf *bytes.Buffer) {
	buf.WriteByte(0x07)
	buf.Write([]byte(RaknetMagic))
	WriteAddress(buf, pk.ServerAddress)
	WriteShort(buf, pk.MtuSize)
	WriteLong(buf, pk.ClientID)
}

// OpenConnectionReply2 is a packet used in Raknet.
type OpenConnectionReply2 struct {
	ServerID      uint64
	ClientAddress *net.UDPAddr
	MtuSize       uint16
}

// Read implements RaknetPacket interfaces.
func (pk *OpenConnectionReply2) Read(buf *bytes.Buffer) {
	buf.Next(16)
	pk.ServerID = ReadLong(buf)
	pk.ClientAddress = ReadAddress(buf)
	pk.MtuSize = ReadShort(buf)
}

// Handle implements RaknetPacket interfaces.
func (pk *OpenConnectionReply2) Handle(session *Session) {}

// Write implements RaknetPacket interfaces.
func (pk *OpenConnectionReply2) Write(buf *bytes.Buffer) {
	buf.WriteByte(0x08)
	buf.Write([]byte(RaknetMagic))
	WriteLong(buf, pk.ServerID)
	WriteAddress(buf, pk.ClientAddress)
	WriteShort(buf, pk.MtuSize)
	WriteByte(buf, 0)
	return
}

// GeneralDataPacket is a packet used in Raknet.
type GeneralDataPacket struct {
	SeqNumber uint32
	Packets   []*EncapsulatedPacket
}

// Read implements RaknetPacket interfaces.
func (pk *GeneralDataPacket) Read(buf *bytes.Buffer) {
	dp := new(DataPacket)
	dp.Buffer = buf
	/*
		log.Println("======= DataPacket dump =======")
		log.Println(hex.Dump(dp.Byte()))
	*/
	dp.Decode()
	pk.SeqNumber = dp.SeqNumber
	pk.Packets = dp.Packets
}

// Handle implements RaknetPacket interfaces.
func (pk *GeneralDataPacket) Handle(session *Session) {
	if pk.SeqNumber < session.windowBorder[0] || pk.SeqNumber >= session.windowBorder[1] {
		return
	}
	session.packetWindow[pk.SeqNumber] = true
	session.ackQueue[pk.SeqNumber] = true
	diff := pk.SeqNumber - session.lastSeq
	if diff != 1 {
		for i := session.lastSeq + 1; i < pk.SeqNumber; i++ {
			if _, ok := session.packetWindow[i]; !ok {
				// log.Println("Seqnumber", i, "is missing. Adding to NACK queue.")
				session.nackQueue[i] = true
			}
		}
	}
	if diff >= 1 {
		session.lastSeq = pk.SeqNumber
		session.windowBorder[0] += diff
		session.windowBorder[1] += diff
		for _, pk := range pk.Packets {
			session.preEncapsulated(pk)
		}
	}
}

// Write implements RaknetPacket interfaces.
func (pk *GeneralDataPacket) Write(buf *bytes.Buffer) {
	dp := new(DataPacket)
	dp.SeqNumber = pk.SeqNumber
	dp.Packets = pk.Packets
	dp.Encode()
	io.Copy(dp.Buffer, buf)
}

// Ack is a packet used in Raknet.
type Ack struct {
	Seqs []uint32
}

// Read implements RaknetPacket interfaces.
func (pk *Ack) Read(buf *bytes.Buffer) {
	pk.Seqs = DecodeAck(buf)
}

// Handle implements RaknetPacket interfaces.
func (pk *Ack) Handle(session *Session) {
	for _, seq := range pk.Seqs {
		if _, ok := session.recovery[seq]; ok {
			delete(session.recovery, seq)
		}
	}
}

// Write implements RaknetPacket interfaces.
func (pk *Ack) Write(buf *bytes.Buffer) {
	// Unused, should be directly sent on session.
}

// Nack is a packet used in Raknet.
type Nack struct {
	Seqs []uint32
}

// Read implements RaknetPacket interfaces.
func (pk *Nack) Read(buf *bytes.Buffer) {
	pk.Seqs = DecodeAck(buf)
}

// Handle implements RaknetPacket interfaces.
func (pk *Nack) Handle(session *Session) {
	for _, seq := range pk.Seqs {
		if _, ok := session.nackQueue[seq]; ok {
			delete(session.nackQueue, seq)
		}
	}
}

// Write implements RaknetPacket interfaces.
func (pk *Nack) Write(buf *bytes.Buffer) {
	// Unused, should be directly sent on session.
}

// ClientConnect is a packet used in Raknet.
type ClientConnect struct {
	ClientID    uint64
	SendPing    uint64
	UseSecurity bool
}

// Read implements RaknetPacket interfaces.
func (pk *ClientConnect) Read(buf *bytes.Buffer) {
	pk.ClientID = ReadLong(buf)
	pk.SendPing = ReadLong(buf)
	pk.UseSecurity = ReadByte(buf) > 0
}

// Handle implements RaknetPacket interfaces.
func (pk *ClientConnect) Handle(session *Session) {
	if session.Status != 2 {
		return
	}
	buf := new(bytes.Buffer)
	p := &ServerHandshake{
		Address:         session.Address,
		SystemAddresses: AddressTemplate,
		SendPing:        pk.SendPing,
		SendPong:        pk.SendPing + 1000,
	}
	p.Write(buf)
	session.sendEncapsulatedDirect(&EncapsulatedPacket{Buffer: buf})
}

// Write implements RaknetPacket interfaces.
func (pk *ClientConnect) Write(buf *bytes.Buffer) {
	buf.WriteByte(0x09)
	WriteLong(buf, pk.ClientID)
	WriteLong(buf, pk.SendPing)
	WriteByte(buf, func() byte {
		if pk.UseSecurity {
			return 1
		}
		return 0
	}())
}

// ClientDisconnect is a packet used in Raknet.
type ClientDisconnect struct{}

// Read implements RaknetPacket interfaces.
func (pk *ClientDisconnect) Read(buf *bytes.Buffer) {}

// Handle implements RaknetPacket interfaces.
func (pk *ClientDisconnect) Handle(session *Session) { session.Close("client disconnect") }

// Write implements RaknetPacket interfaces.
func (pk *ClientDisconnect) Write(buf *bytes.Buffer) {}

// ClientHandshake is a packet used in Raknet.
type ClientHandshake struct {
	Address            *net.UDPAddr
	SystemAddresses    []*net.UDPAddr
	SendPing, SendPong uint64
}

// Read implements RaknetPacket interfaces.
func (pk *ClientHandshake) Read(buf *bytes.Buffer) {
	pk.Address = ReadAddress(buf)
	addrs := make([]*net.UDPAddr, 10)
	for i := 0; i < 10; i++ {
		addrs[i] = ReadAddress(buf)
	}
	pk.SystemAddresses = addrs
	pk.SendPing = ReadLong(buf)
	pk.SendPong = ReadLong(buf)
}

// Handle implements RaknetPacket interfaces.
func (pk *ClientHandshake) Handle(session *Session) {
	log.Println("Client connected successfully!")
	session.Status = 3
	session.connComplete()
}

// Write implements RaknetPacket interfaces.
func (pk *ClientHandshake) Write(buf *bytes.Buffer) {
	buf.WriteByte(0x13)
	WriteAddress(buf, pk.Address)
	for _, addr := range pk.SystemAddresses {
		WriteAddress(buf, addr)
	}
	WriteLong(buf, pk.SendPing)
	WriteLong(buf, pk.SendPong)
}

// ServerHandshake is a packet used in Raknet.
type ServerHandshake struct {
	Address            *net.UDPAddr
	SystemAddresses    []*net.UDPAddr
	SendPing, SendPong uint64
}

// Read implements RaknetPacket interfaces.
func (pk *ServerHandshake) Read(buf *bytes.Buffer) {
	pk.Address = ReadAddress(buf)
	buf.Next(1) // Unknown
	addrs := make([]*net.UDPAddr, 10)
	for i := 0; i < 10; i++ {
		addrs[0] = ReadAddress(buf)
	}
	pk.SystemAddresses = addrs
	pk.SendPing = ReadLong(buf)
	pk.SendPong = ReadLong(buf)
}

// Handle implements RaknetPacket interfaces.
func (pk *ServerHandshake) Handle(session *Session) {}

// Write implements RaknetPacket interfaces.
func (pk *ServerHandshake) Write(buf *bytes.Buffer) {
	buf.WriteByte(0x10)
	WriteAddress(buf, pk.Address)
	WriteByte(buf, 0) // Unknown
	for _, addr := range pk.SystemAddresses {
		WriteAddress(buf, addr)
	}
	WriteLong(buf, pk.SendPing)
	WriteLong(buf, pk.SendPong)
	return
}

// Ping is a packet used in Raknet.
type Ping struct {
	PingID uint64
}

// Read implements RaknetPacket interfaces.
func (pk *Ping) Read(buf *bytes.Buffer) {
	pk.PingID = ReadLong(buf)
}

// Handle implements RaknetPacket interfaces.
func (pk *Ping) Handle(session *Session) {
	buf := new(bytes.Buffer)
	p := &Pong{PingID: pk.PingID}
	p.Write(buf)
	session.sendEncapsulatedDirect(&EncapsulatedPacket{Buffer: buf})
}

// Write implements RaknetPacket interfaces.
func (pk *Ping) Write(buf *bytes.Buffer) {
	buf.WriteByte(0x00)
	WriteLong(buf, pk.PingID)
	return
}

// Pong is a packet used in Raknet.
type Pong struct {
	PingID uint64
}

// Read implements RaknetPacket interfaces.
func (pk *Pong) Read(buf *bytes.Buffer) {
	pk.PingID = ReadLong(buf)
	return
}

// Handle implements RaknetPacket interfaces.
func (pk *Pong) Handle(session *Session) {
	if session.pingTries > 0 {
		session.timeout.Reset(timeout)
		session.pingTries = 0
	}
	return
}

// Write implements RaknetPacket interfaces.
func (pk *Pong) Write(buf *bytes.Buffer) {
	buf.WriteByte(0x03)
	WriteLong(buf, pk.PingID)
	return
}
