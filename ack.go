package highmc

import (
	"bytes"
	"sort"
)

type ackTable []uint32

func (t ackTable) Len() int           { return len(t) }
func (t ackTable) Swap(i, j int)      { t[i], t[j] = t[j], t[i] }
func (t ackTable) Less(i, j int) bool { return t[i] < t[j] }

// EncodeAck packs packet sequence numbers into Raknet acknowledgment format.
func EncodeAck(t ackTable) (b *bytes.Buffer) {
	b = new(bytes.Buffer)
	sort.Sort(t)
	packets := t.Len()
	records := uint16(0)
	if packets > 0 {
		pointer := 1
		start, last := t[0], t[0]
		for pointer < packets {
			current := t[pointer]
			pointer++
			diff := current - last
			if diff == 1 {
				last = current
			} else if diff > 1 {
				if start == last {
					WriteByte(b, 1)
					WriteLTriad(b, start)
					last = current
					start = last
				} else {
					WriteByte(b, 0)
					WriteLTriad(b, start)
					WriteLTriad(b, last)
					last = current
					start = last
				}
				records++
			}
		}
		if start == last {
			WriteByte(b, 1)
			WriteLTriad(b, start)
		} else {
			WriteByte(b, 0)
			WriteLTriad(b, start)
			WriteLTriad(b, last)
		}
		records++
	}
	tmp := new(bytes.Buffer)
	WriteShort(tmp, records)
	tmp.Write(b.Bytes())
	b = tmp
	return
}

// DecodeAck unpacks packet sequence numbers from given
func DecodeAck(b *bytes.Buffer) (t []uint32) {
	var records uint16
	records = ReadShort(b)
	count := 0
	for i := 0; uint16(i) < records && b.Len() > 0 && count < 4096; i++ {
		if f := ReadByte(b); f == 0 {
			start := ReadLTriad(b)
			last := ReadLTriad(b)
			if (last - start) > 512 {
				last = start + 512
			}
			for c := start; c <= last; c++ {
				t = append(t, c)
				count++
			}
		} else {
			p := ReadLTriad(b)
			t = append(t, p)
			count++
		}
	}
	return
}
