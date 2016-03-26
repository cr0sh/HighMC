package highmc

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"net"
)

// Overflow is an error indicates the reader could not read as you requested.
type Overflow struct {
	Need int
	Got  int
}

// Error implements the error interface.
func (e Overflow) Error() string {
	return fmt.Sprintf("Overflow: Needed %d, got %d", e.Need, e.Got)
}

// StringOverflow represents the given string is too long for write
type StringOverflow struct {
	Length int
}

// Error implements the error interface.
func (err StringOverflow) Error() string {
	return fmt.Sprintf("String too long: Given string is %d characters long, it overflows uint16(65535)", err.Length)
}

// Read reads n bytes of data from buf. If buf returns smaller slice than n, returns OverFlow.
func Read(rd io.Reader, n int) (b []byte, err error) {
	b = make([]byte, n)
	if rn, err := rd.Read(b); err != nil {
		return b, err
	} else if rn != n {
		err = Overflow{
			Need: n,
			Got:  rn,
		}
		return b, err
	}
	return
}

// ReadAny reads appropriate type from given reference value.
func ReadAny(rd io.Reader, p interface{}) {
	switch p.(type) {
	case *bool:
		*p.(*bool) = ReadBool(rd)
	case *byte:
		*p.(*byte) = ReadByte(rd)
	case *uint16:
		*p.(*uint16) = ReadShort(rd)
	case *uint32:
		*p.(*uint32) = ReadInt(rd)
	case *uint64:
		*p.(*uint64) = ReadLong(rd)
	case *float32:
		*p.(*float32) = ReadFloat(rd)
	case *float64:
		*p.(*float64) = ReadDouble(rd)
	case *string:
		*p.(*string) = ReadString(rd)
	case *net.UDPAddr:
		var addr *net.UDPAddr
		addr = ReadAddress(rd)
		*p.(*net.UDPAddr) = *addr
	case **net.UDPAddr:
		*p.(**net.UDPAddr) = ReadAddress(rd)
	case byte, uint16, uint32,
		uint64, float32, float64, string, net.UDPAddr:
		panic("ReadAny requires reference type")
	default:
		panic("Unsupported type for ReadAny")
	}
}

// BatchRead batches ReadAny from given reference pointers.
func BatchRead(rd io.Reader, p ...interface{}) {
	for _, pp := range p {
		ReadAny(rd, pp)
	}
}

// ReadBool reads boolean from buffer.
func ReadBool(rd io.Reader) bool {
	b, err := Read(rd, 1)
	if err != nil {
		panic(err)
	}
	return b[0] > 0
}

// ReadByte reads unsigned byte from buffer.
func ReadByte(rd io.Reader) byte {
	b, err := Read(rd, 1)
	if err != nil {
		panic(err)
	}
	return b[0]
}

// ReadShort reads unsigned short from buffer.
func ReadShort(rd io.Reader) uint16 {
	b, err := Read(rd, 2)
	if err != nil {
		panic(err)
	}
	return uint16(b[0])<<8 | uint16(b[1])
}

// ReadLShort reads unsigned little-endian short from buffer.
func ReadLShort(rd io.Reader) uint16 {
	b, err := Read(rd, 2)
	if err != nil {
		panic(err)
	}
	return uint16(b[1])<<8 | uint16(b[0])
}

// ReadInt reads unsigned int from buffer.
func ReadInt(rd io.Reader) uint32 {
	b, err := Read(rd, 4)
	if err != nil {
		panic(err)
	}
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
}

// ReadLInt reads unsigned little-endian int from buffer.
func ReadLInt(rd io.Reader) uint32 {
	b, err := Read(rd, 4)
	if err != nil {
		panic(err)
	}
	return uint32(b[3])<<24 | uint32(b[2])<<16 | uint32(b[1])<<8 | uint32(b[0])
}

// ReadLong reads unsigned long from buffer.
func ReadLong(rd io.Reader) uint64 {
	b, err := Read(rd, 8)
	if err != nil {
		panic(err)
	}
	return uint64(b[0])<<56 | uint64(b[1])<<48 |
		uint64(b[2])<<40 | uint64(b[3])<<32 |
		uint64(b[4])<<24 | uint64(b[5])<<16 |
		uint64(b[6])<<8 | uint64(b[7])
}

// ReadLLong reads unsigned little-endian long from buffer.
func ReadLLong(rd io.Reader) uint64 {
	b, err := Read(rd, 8)
	if err != nil {
		panic(err)
	}
	return uint64(b[7])<<56 | uint64(b[6])<<48 |
		uint64(b[5])<<40 | uint64(b[4])<<32 |
		uint64(b[3])<<24 | uint64(b[2])<<16 |
		uint64(b[1])<<8 | uint64(b[0])
}

// ReadFloat reads 32-bit float from buffer.
func ReadFloat(rd io.Reader) float32 {
	r := ReadInt(rd)
	return math.Float32frombits(r)
}

// ReadDouble reads 64-bit float from buffer.
func ReadDouble(rd io.Reader) float64 {
	r := ReadLong(rd)
	return math.Float64frombits(r)
}

// ReadTriad reads unsigned 3-bytes triad from buffer.
func ReadTriad(rd io.Reader) uint32 {
	b, err := Read(rd, 3)
	if err != nil {
		panic(err)
	}
	return uint32(b[0])<<16 | uint32(b[1])<<8 | uint32(b[2])
}

// ReadLTriad reads unsigned little-endian 3-bytes triad from buffer.
func ReadLTriad(rd io.Reader) uint32 {
	b, err := Read(rd, 3)
	if err != nil {
		panic(err)
	}
	return uint32(b[2])<<16 | uint32(b[1])<<8 | uint32(b[0])
}

// ReadString reads string from buffer.
func ReadString(rd io.Reader) (str string) {
	b, err := Read(rd, int(ReadShort(rd)))
	if err != nil {
		panic(err)
	}
	return string(b)
}

// ReadAddress reads IP address/port from buffer.
func ReadAddress(rd io.Reader) (addr *net.UDPAddr) {
	v := ReadByte(rd)
	if v != 4 {
		panic(fmt.Sprintf("ReadAddress got unsupported IP version %d", v))
	}
	b, err := Read(rd, 4)
	if err != nil {
		panic(err)
	}
	p := ReadShort(rd)
	return &net.UDPAddr{
		IP:   append([]byte{b[0] ^ 0xff}, b[1]^0xff, b[2]^0xff, b[3]^0xff),
		Port: int(p),
	}
}

// Write writes given byte array to buffer.
func Write(wr io.Writer, b []byte) error {
	n, err := wr.Write(b)
	if err == nil && n != len(b) {
		err = Overflow{
			Need: len(b),
			Got:  n,
		}
	}
	return err
}

// WriteAny writes appropriate type from given interface{} value to buffer.
func WriteAny(wr io.Writer, p interface{}) {
	switch p.(type) {
	case bool:
		WriteBool(wr, p.(bool))
	case byte:
		WriteByte(wr, p.(byte))
	case uint16:
		WriteShort(wr, p.(uint16))
	case uint32:
		WriteInt(wr, p.(uint32))
	case uint64:
		WriteLong(wr, p.(uint64))
	case float32:
		WriteFloat(wr, p.(float32))
	case float64:
		WriteDouble(wr, p.(float64))
	case string:
		WriteString(wr, p.(string))
	case []byte:
		Write(wr, p.([]byte))
	case *bool:
		WriteBool(wr, *p.(*bool))
	case *byte:
		WriteByte(wr, *p.(*byte))
	case *uint16:
		WriteShort(wr, *p.(*uint16))
	case *uint32:
		WriteInt(wr, *p.(*uint32))
	case *uint64:
		WriteLong(wr, *p.(*uint64))
	case *float32:
		WriteFloat(wr, *p.(*float32))
	case *float64:
		WriteDouble(wr, *p.(*float64))
	case *string:
		WriteString(wr, *p.(*string))
	case *[]byte:
		Write(wr, *p.(*[]byte))
	case *net.UDPAddr:
		WriteAddress(wr, p.(*net.UDPAddr))
	}
}

// BatchWrite batches WriteAny from given values.
func BatchWrite(wr io.Writer, p ...interface{}) {
	for _, pp := range p {
		WriteAny(wr, pp)
	}
}

// WriteBool writes boolean to buffer.
func WriteBool(wr io.Writer, n bool) {
	WriteByte(wr, func() byte {
		if n {
			return 1
		}
		return 0
	}())
}

// WriteByte writes unsigned byte to buffer.
func WriteByte(wr io.Writer, n byte) {
	if err := Write(wr, []byte{n}); err != nil {
		panic(err)
	}
}

// WriteShort writes unsigned short to buffer.
func WriteShort(wr io.Writer, n uint16) {
	if err := Write(wr, []byte{byte(n >> 8), byte(n)}); err != nil {
		panic(err)
	}
}

// WriteLShort writes unsigned little-endian short to buffer.
func WriteLShort(wr io.Writer, n uint16) {
	if err := Write(wr, []byte{byte(n), byte(n >> 8)}); err != nil {
		panic(err)
	}
}

// WriteInt writes unsigned int to buffer.
func WriteInt(wr io.Writer, n uint32) {
	if err := Write(wr, []byte{byte(n >> 24), byte(n >> 16), byte(n >> 8), byte(n)}); err != nil {
		panic(err)
	}
}

// WriteLInt writes unsigned little-endian int to buffer.
func WriteLInt(wr io.Writer, n uint32) {
	if err := Write(wr, []byte{byte(n), byte(n >> 8), byte(n >> 16), byte(n >> 24)}); err != nil {
		panic(err)
	}
}

// WriteLong writes unsigned long to buffer.
func WriteLong(wr io.Writer, n uint64) {
	if err := Write(wr, []byte{
		byte(n >> 56), byte(n >> 48),
		byte(n >> 40), byte(n >> 32),
		byte(n >> 24), byte(n >> 16),
		byte(n >> 8), byte(n),
	}); err != nil {
		panic(err)
	}
}

// WriteLLong writes unsigned little-endian long to buffer.
func WriteLLong(wr io.Writer, n uint64) {
	if err := Write(wr, []byte{
		byte(n), byte(n >> 8),
		byte(n >> 16), byte(n >> 24),
		byte(n >> 32), byte(n >> 40),
		byte(n >> 48), byte(56),
	}); err != nil {
		panic(err)
	}
}

// WriteFloat writes 32-bit float to buffer.
func WriteFloat(wr io.Writer, f float32) {
	WriteInt(wr, math.Float32bits(f))
}

// WriteDouble writes 64-bit float to buffer.
func WriteDouble(wr io.Writer, f float64) {
	WriteLong(wr, math.Float64bits(f))
}

// WriteTriad writes unsigned 3-bytes triad to buffer.
func WriteTriad(wr io.Writer, n uint32) {
	if err := Write(wr, []byte{byte(n >> 16), byte(n >> 8), byte(n)}); err != nil {
		panic(err)
	}
}

// WriteLTriad writes unsigned little-endian 3-bytes triad to buffer.
func WriteLTriad(wr io.Writer, n uint32) error {
	return Write(wr, []byte{byte(n), byte(n >> 8), byte(n >> 16)})
}

// WriteString writes string to buffer.
func WriteString(wr io.Writer, s string) {
	if len(s) > 65535 {
		panic(StringOverflow{
			Length: len(s),
		})
	}
	WriteShort(wr, uint16(len(s)))
	Write(wr, []byte(s))
}

// WriteAddress writes net.UDPAddr address to buffer.
func WriteAddress(wr io.Writer, i *net.UDPAddr) {
	WriteByte(wr, 4)
	for _, v := range i.IP.To4() {
		WriteByte(wr, v^0xff)
	}
	WriteShort(wr, uint16(i.Port))
}

// Dump prints hexdump for given
func Dump(buf *bytes.Buffer) {
	fmt.Print(hex.Dump(buf.Bytes()))
}
