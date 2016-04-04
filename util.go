package highmc

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"math"
	"reflect"
	"sort"
)

// Try runs tryFunc, catches panic, and executes panicHandle with recovered panic.
func Try(tryFunc func(), panicHandle func(interface{})) {
	defer func() {
		if r := recover(); r != nil {
			panicHandle(r)
		}
	}()
	tryFunc()
	return
}

// Safe runs panicFunc, recovers panic if exists, and returns as error.
func Safe(panicFunc func()) error {
	var err error
	func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("%v", r)
			}
		}()
		panicFunc()
	}()
	return err
}

// DecodeDeflate returns decompressed data of given byte slice.
func DecodeDeflate(b []byte) (*bytes.Buffer, error) {
	r, err := zlib.NewReader(bytes.NewBuffer(b))
	if err != nil {
		return nil, err
	}
	output := new(bytes.Buffer)
	io.Copy(output, r)
	r.Close()
	return output, nil
}

// EncodeDeflate returns compressed data of given byte slice.
func EncodeDeflate(b *bytes.Buffer) []byte {
	o := new(bytes.Buffer)
	w := zlib.NewWriter(o)
	io.Copy(w, b)
	w.Close()
	return o.Bytes()
}

// GetSortedKeys will return a sorted slice of 'uint' keys from given map.
func GetSortedKeys(m interface{}) []int {
	mm := reflect.ValueOf(m)
	keys := make([]int, len(mm.MapKeys()))
	i := 0
	for _, k := range mm.MapKeys() {
		keys[i] = int(k.Uint())
		i++
	}
	sort.Ints(keys)
	return keys
}

// Face/Side indicators
const (
	SideDown  = iota // Y-
	SideUp           // Y+
	SideNorth        // Z-
	SideSouth        // Z+
	SideWest         // X-
	SideEast         // X+
)

// Direction indicators
const (
	South = iota // Z+
	West         // X-
	North        // Z-
	East         // X+
)

// Vector2 is a X-Y vector, containing 2nd-dimension position.
type Vector2 struct {
	X, Y float32
}

// Distance calculates the distance between given vector.
func (v Vector2) Distance(to Vector2) float32 {
	return float32(math.Sqrt(float64((to.X-v.X)*(to.X-v.X) + (to.Y-v.Y)*(to.Y-v.Y))))
}

// Vector3 converts Vector2 to Vector3, setting X, Y as set before, but leaving Z zero.
func (v Vector2) Vector3() Vector3 {
	return Vector3{
		X: v.X,
		Y: v.Y,
	}
}

// Vector3 is a X-Y-Z vector, containing 3rd-dimension position.
type Vector3 struct {
	X, Y, Z float32
}

// Distance calculates the distance between given vector.
func (v Vector3) Distance(to Vector3) float32 {
	return float32(math.Sqrt(float64((to.X-v.X)*(to.X-v.X) + (to.Y-v.Y)*(to.Y-v.Y) + (to.Z-v.Z)*(to.Z-v.Z))))
}
