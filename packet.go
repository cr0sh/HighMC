package highmc

import (
	"bytes"
	"net"
	"time"
)

// Packet is a struct which contains binary buffer, address, and send time.
type Packet struct {
	*bytes.Buffer
	Address *net.UDPAddr
	Recycle bool
}

// NewPacket creates new packet with given packet id.
func NewPacket(pid byte) Packet {
	return Packet{Pool.NewBuffer([]byte{pid}), new(net.UDPAddr), false}
}

// EncapsulatedPacket is a struct, containing more values for decoding/encoding encapsualted packets.
type EncapsulatedPacket struct {
	*bytes.Buffer
	Reliability  byte
	HasSplit     bool
	MessageIndex uint32 // LE Triad
	OrderIndex   uint32 // LE Triad
	OrderChannel byte
	SplitCount   uint32
	SplitID      uint16
	SplitIndex   uint32
}

// NewEncapsulated returns decoded EncapsulatedPacket struct from given binary.
// Do NOT set buf with *Packet struct. It could cause panic.
func NewEncapsulated(buf *bytes.Buffer) (ep *EncapsulatedPacket) {
	ep = new(EncapsulatedPacket)
	flags := ReadByte(buf)
	ep.Reliability = flags >> 5
	ep.HasSplit = (flags>>4)&1 > 0
	l := uint32(ReadShort(buf))
	length := l >> 3
	if l&7 != 0 {
		length++
	}
	if ep.Reliability > 0 {
		if ep.Reliability >= 2 && ep.Reliability != 5 {
			ep.MessageIndex = ReadLTriad(buf)
		}
		if ep.Reliability <= 4 && ep.Reliability != 2 {
			ep.OrderIndex = ReadLTriad(buf)
			ep.OrderChannel = ReadByte(buf)
		}
	}
	if ep.HasSplit {
		ep.SplitCount = ReadInt(buf)
		ep.SplitID = ReadShort(buf)
		ep.SplitIndex = ReadInt(buf)
	}
	b, err := Read(buf, int(length))
	if err != nil {
		panic(err.Error())
	}
	ep.Buffer = Pool.NewBuffer(b)
	return
}

// TotalLen returns total binary length of EncapsulatedPacket.
func (ep *EncapsulatedPacket) TotalLen() int {
	return 3 + ep.Len() + func() int {
		return func() int {
			if ep.Reliability >= 2 && ep.Reliability != 5 {
				return 3
			}
			return 0
		}() + func() int {
			if ep.Reliability != 0 && ep.Reliability <= 4 && ep.Reliability != 2 {
				return 4
			}
			return 0
		}()
	}() + func() int {
		if ep.HasSplit {
			return 10
		}
		return 0
	}()
}

// Bytes returns encoded binary from EncapsulatedPacket struct options.
func (ep *EncapsulatedPacket) Bytes() (buf *bytes.Buffer) {
	buf = Pool.NewBuffer(nil)
	WriteByte(buf, ep.Reliability<<5|func() byte {
		if ep.HasSplit {
			return 1 << 4
		}
		return 0
	}())
	WriteShort(buf, uint16(ep.Len())<<3)
	if ep.Reliability > 0 {
		Write(buf, func() []byte {
			buf := Pool.NewBuffer(nil)
			if ep.Reliability >= 2 && ep.Reliability != 5 {
				WriteLTriad(buf, ep.MessageIndex)
			}
			if ep.Reliability <= 4 && ep.Reliability != 2 {
				WriteLTriad(buf, ep.OrderIndex)
				WriteByte(buf, ep.OrderChannel)
			}
			return buf.Bytes()
		}())
	}
	if ep.HasSplit {
		WriteInt(buf, ep.SplitCount)
		WriteShort(buf, ep.SplitID)
		WriteInt(buf, ep.SplitIndex)
	}
	Write(buf, ep.Buffer.Bytes())
	return
}

// DataPacket is a packet struct, containing Raknet data packet fields.
type DataPacket struct {
	*bytes.Buffer
	Head      byte
	SendTime  time.Time
	SeqNumber uint32 // LE Triad
	Packets   []*EncapsulatedPacket
}

// Decode decodes buffer to struct fields and decapsulates all packets.
func (dp *DataPacket) Decode() {
	// dp.Head = ReadByte(dp.Buffer)
	dp.SeqNumber = ReadLTriad(dp.Buffer)
	for dp.Len() > 0 {
		ep := NewEncapsulated(dp.Buffer)
		dp.Packets = append(dp.Packets, ep)
	}
	return
}

// TotalLen returns total buffer length of data packet.
func (dp *DataPacket) TotalLen() int {
	length := 4
	for _, d := range dp.Packets {
		length += d.TotalLen()
	}
	return length
}

// Encode encodes fields and packets to buffer.
func (dp *DataPacket) Encode() {
	dp.Buffer = Pool.NewBuffer(nil)
	WriteByte(dp.Buffer, dp.Head)
	WriteLTriad(dp.Buffer, dp.SeqNumber)
	for _, ep := range dp.Packets {
		Write(dp.Buffer, ep.Bytes().Bytes())
	}
	return
}
