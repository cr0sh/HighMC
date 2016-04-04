package highmc

// FIXME
/*
type blockUpdateHandler func(int32, int32, int32, Block, *Level) []BlockRecord

var blockHandlerMap = map[byte]blockUpdateHandler{
	byte(Torch): func(x, y, z int32, block Block, lv *Level) []BlockRecord {
		sx, sy, sz := x, y, z
		switch block.Meta {
		case 0:
			sy--
		case 1:
			sx--
		case 2:
			sx++
		case 3:
			sz--
		case 4:
			sz++
		case 5:
			sy--
		}
		dep := lv.GetBlock(sx, sy, sz)
		if dep == byte(Air) {
			return []BlockRecord{
				lv.SetRecord(x, y, z, Block{ID: byte(Air)}),
			}
		}
		return nil
	},
	byte(Cactus): func(x, y, z int32, block Block, lv *Level) []BlockRecord {
		if lv.GetBlock(x+1, y, z) != byte(Air) ||
			lv.GetBlock(x, y, z+1) != byte(Air) ||
			lv.GetBlock(x, y, z-1) != byte(Air) ||
			lv.GetBlock(x-1, y, z) != byte(Air) {
			return []BlockRecord{
				lv.SetRecord(x, y, z, Block{ID: byte(Air)}),
			}
		}
		below := lv.GetBlock(x, y-1, z)
		if below == byte(Sand) {
			if !(lv.GetBlock(x, y-1, z) == byte(Sand) &&
				lv.GetBlock(x, y-1, z) == byte(Sand) &&
				lv.GetBlock(x, y-1, z) == byte(Sand) &&
				lv.GetBlock(x, y-1, z) == byte(Sand) &&
				lv.GetBlock(x, y-1, z) == byte(Sand) &&
				lv.GetBlock(x, y-1, z) == byte(Sand) &&
				lv.GetBlock(x, y-1, z) == byte(Sand) &&
				lv.GetBlock(x, y-1, z) == byte(Sand)) {
				return []BlockRecord{
					lv.SetRecord(x, y, z, Block{ID: byte(Air)}),
				}
			}
		} else if below == byte(Cactus) {
			return nil
		}
		return []BlockRecord{
			lv.SetRecord(x, y, z, Block{ID: byte(Air)}),
		}
	},
}

// NeedUpdate determines the given block should handle update.
func NeedUpdate(ID byte) bool {
	_, ok := blockHandlerMap[ID]
	return ok
}
*/
