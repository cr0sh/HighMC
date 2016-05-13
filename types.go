package highmc

import (
	"fmt"
	"io"

	"github.com/minero/minero/proto/nbt"
)

// Block contains block data for each level positions
type Block struct {
	ID   byte
	Meta byte
}

// Item converts block to item struct.
func (b Block) Item() Item {
	return Item{
		ID:   ID(b.ID),
		Meta: uint16(b.Meta),
	}
}

// ChunkPos is a type for identifying chunks by x-z coordinate.
type ChunkPos struct {
	X, Z int32
}

// GetChunkPos extracts ChunkPos from BlockPos.
func GetChunkPos(p BlockPos) ChunkPos {
	return ChunkPos{
		X: p.X >> 4,
		Z: p.Z >> 4,
	}
}

// ChunkDelivery is a type for passing full chunk data to players.
type ChunkDelivery struct {
	ChunkPos
	Chunk *Chunk
}

// Chunk contains block data for each MCPE level chunks.
// Each chunk holds 16*16*128 blocks, and consumes at least 83208 bytes of memory.
//
// A zero value for Chunk is a valid value.
type Chunk struct {
	BlockData    [16 * 16 * 128]byte
	MetaData     [16 * 16 * 64]byte // Nibbles
	LightData    [16 * 16 * 64]byte // Nibbles
	SkyLightData [16 * 16 * 64]byte // Nibbles
	HeightMap    [16 * 16]byte
	BiomeData    [16 * 16 * 4]byte // Uints

	Position ChunkPos
	Refs     uint64
}

// FallbackChunk is a chunk to be returned if level provider fails to load chunk from file.
var FallbackChunk = *new(Chunk)

// CopyFrom gets everything from given chunk, and writes to the chunk instance.
func (c *Chunk) CopyFrom(chunk Chunk) {
	copy(c.BlockData[:], chunk.BlockData[:])
	copy(c.MetaData[:], chunk.MetaData[:])
	copy(c.LightData[:], chunk.LightData[:])
	copy(c.SkyLightData[:], chunk.SkyLightData[:])
	copy(c.HeightMap[:], chunk.HeightMap[:])
	copy(c.BiomeData[:], chunk.BiomeData[:])
}

// GetBlock returns block ID at given coordinates.
func (c *Chunk) GetBlock(x, y, z byte) byte {
	return c.BlockData[uint16(y)<<8|uint16(z)<<4|uint16(x)]
}

// SetBlock sets block ID at given coordinates.
func (c *Chunk) SetBlock(x, y, z, id byte) {
	c.BlockData[uint16(y)<<8|uint16(z)<<4|uint16(x)] = id
	if id != 0 && y > c.GetHeightMap(x, z) {
		c.SetHeightMap(x, z, y)
	}
	if id == 0 && y == c.GetHeightMap(x, z) {
		c.getHeight(x, z)
	}
}

// GetBlockMeta returns block meta at given coordinates.
func (c *Chunk) GetBlockMeta(x, y, z byte) byte {
	if x&1 == 0 {
		return c.MetaData[uint16(y)<<7|uint16(z)<<3|uint16(x)>>1] & 0x0f
	}
	return c.MetaData[uint16(y)<<7|uint16(z)<<3|uint16(x)>>1] >> 4
}

// SetBlockMeta sets block meta at given coordinates.
func (c *Chunk) SetBlockMeta(x, y, z, id byte) {
	b := c.MetaData[uint16(y)<<7|uint16(z)<<3|uint16(x)>>1]
	if x&1 == 0 {
		c.MetaData[uint16(y)<<7|uint16(z)<<3|uint16(x)>>1] = (b & 0xf0) | (id & 0x0f)
	} else {
		c.MetaData[uint16(y)<<7|uint16(z)<<3|uint16(x)>>1] = (id&0xf)<<4 | (b & 0x0f)
	}
}

// GetBlockLight returns block light level at given coordinates.
func (c *Chunk) GetBlockLight(x, y, z byte) byte {
	if x&1 == 0 {
		return c.LightData[uint16(y)<<7|uint16(z)<<3|uint16(x)>>1] & 0x0f
	}
	return c.LightData[uint16(y)<<7|uint16(z)<<3|uint16(x)>>1] >> 4
}

// SetBlockLight sets block light level at given coordinates.
func (c *Chunk) SetBlockLight(x, y, z, id byte) {
	b := c.LightData[uint16(y)<<7|uint16(z)<<3|uint16(x)>>1]
	if x&1 == 0 {
		c.LightData[uint16(y)<<7|uint16(z)<<3|uint16(x)>>1] = (b & 0xf0) | (id & 0x0f)
	} else {
		c.LightData[uint16(y)<<7|uint16(z)<<3|uint16(x)>>1] = (id&0xf)<<4 | (b & 0x0f)
	}
}

// GetBlockSkyLight returns sky light level at given coordinates.
func (c *Chunk) GetBlockSkyLight(x, y, z byte) byte {
	if x&1 == 0 {
		return c.SkyLightData[uint16(y)<<7|uint16(z)<<3|uint16(x)>>1] & 0x0f
	}
	return c.SkyLightData[uint16(y)<<7|uint16(z)<<3|uint16(x)>>1] >> 4
}

// SetBlockSkyLight sets sky light level at given coordinates.
func (c *Chunk) SetBlockSkyLight(x, y, z, id byte) {
	b := c.SkyLightData[uint16(y)<<7|uint16(z)<<3|uint16(x)>>1]
	if x&1 == 0 {
		c.SkyLightData[uint16(y)<<7|uint16(z)<<3|uint16(x)>>1] = (b & 0xf0) | (id & 0x0f)
	} else {
		c.SkyLightData[uint16(y)<<7|uint16(z)<<3|uint16(x)>>1] = (id&0xf)<<4 | (b & 0x0f)
	}
}

// GetHeightMap returns highest block height on given X-Z coordinates.
func (c *Chunk) GetHeightMap(x, z byte) byte {
	return c.HeightMap[uint16(z)<<4|uint16(x)]
}

// SetHeightMap saves highest block height on given X-Z coordinates.
func (c *Chunk) SetHeightMap(x, z, h byte) {
	c.HeightMap[uint16(z)<<4|uint16(x)] = h
}

// GetBiomeID returns biome ID on given X-Z coordinates.
func (c *Chunk) GetBiomeID(x, z byte) byte {
	return c.BiomeData[uint16(z)<<6|uint16(x)<<2]
}

// SetBiomeID sets biome ID on given X-Z coordinates.
func (c *Chunk) SetBiomeID(x, z, id byte) {
	c.BiomeData[uint16(z)<<6|uint16(x)<<2] = id
}

// GetBiomeColor returns biome color on given X-Z coordinates.
func (c *Chunk) GetBiomeColor(x, z byte) (r, g, b byte) {
	rgb := c.BiomeData[uint16(z)<<6|uint16(x)<<2+1 : uint16(z)<<6|uint16(x)<<2+4]
	return rgb[0], rgb[1], rgb[2]
}

// SetBiomeColor sets biome color on given X-Z coordinates.
func (c *Chunk) SetBiomeColor(x, z, r, g, b byte) {
	offset := uint16(z)<<6 | uint16(x)<<2
	c.BiomeData[offset+1], c.BiomeData[offset+2], c.BiomeData[offset+3] = r, g, b
}

// PopulateHeight populates chunk's block height map.
func (c *Chunk) PopulateHeight() {
	for x := byte(0); x < 16; x++ {
		for z := byte(0); z < 16; z++ {
			c.getHeight(x, z)
		}
	}
}

func (c *Chunk) getHeight(x, z byte) {
	for y := byte(127); y > 0; y-- {
		if c.GetBlock(x, y, z) != 0 {
			c.SetHeightMap(x, z, y)
			return
		}
	}
}

// FullChunkData returns full chunk payload for FullChunkDataPacket. Order is layered.
func (c *Chunk) FullChunkData() []byte {
	buf := Pool.NewBuffer(c.BlockData[:]) // Block ID
	Write(buf, c.MetaData[:])
	Write(buf, c.SkyLightData[:])
	Write(buf, c.LightData[:])
	Write(buf, c.HeightMap[:])
	Write(buf, c.BiomeData[:])
	Write(buf, []byte{0, 0, 0, 0}) // Extra data: NBT length 0
	// No tile entity NBT fields
	return buf.Bytes()
}

// ID represents ID for Minecraft blocks/items.
type ID uint16

// String converts ID to string.
func (id ID) String() string {
	if name, ok := nameMap[id]; ok {
		return name
	}
	return "Unknown"
}

// Block tries to convert item ID to block ID. If fails, it panics.
func (id ID) Block() byte {
	if id >= 256 {
		panic(fmt.Sprintf("item ID %d(%s) overflows byte", uint16(id), id))
	}
	return byte(id)
}

// item/block IDs
const (
	Air                ID = iota
	Stone                 // 1
	Grass                 // 2
	Dirt                  // 3
	Cobblestone           // 4
	Plank                 // 5
	Sapling               // 6
	Bedrock               // 7
	Water                 // 8
	StillWater            // 9
	Lava                  // 10
	StillLava             // 11
	Sand                  // 12
	Gravel                // 13
	GoldOre               // 14
	IronOre               // 15
	CoalOre               // 16
	Log                   // 17
	Leaves                // 18
	Sponge                // 19
	Glass                 // 20
	LapisOre              // 21
	LapisBlock            // 22
	_                     // 23
	Sandstone             // 24
	_                     // 25
	BedBlock              // 26
	_                     // 27
	_                     // 28
	_                     // 29
	Cobweb                // 30
	TallGrass             // 31
	Bush                  // 32
	_                     // 33
	_                     // 34
	Wool                  // 35
	_                     // 36
	Dandelion             // 37
	Poppy                 // 38
	BrownMushroom         // 39
	RedMushroom           // 40
	GoldBlock             // 41
	IronBlock             // 42
	DoubleSlab            // 43
	Slab                  // 44
	Bricks                // 45
	Tnt                   // 46
	Bookshelf             // 47
	MossStone             // 48
	Obsidian              // 49
	Torch                 // 50
	Fire                  // 51
	MonsterSpawner        // 52
	WoodStairs            // 53
	Chest                 // 54
	_                     // 55
	DiamondOre            // 56
	DiamondBlock          // 57
	CraftingTable         // 58
	WheatBlock            // 59
	Farmland              // 60
	Furnace               // 61
	BurningFurnace        // 62
	SignPost              // 63
	DoorBlock             // 64
	Ladder                // 65
	_                     // 66
	CobbleStairs          // 67
	WallSign              // 68
	_                     // 69
	_                     // 70
	IronDoorBlock         // 71
	_                     // 72
	RedstoneOre           // 73
	GlowingRedstoneOre    // 74
	_                     // 75
	_                     // 76
	_                     // 77
	Snow                  // 78
	Ice                   // 79
	SnowBlock             // 80
	Cactus                // 81
	ClayBlock             // 82
	Reeds                 // 83
	_                     // 84
	Fence                 // 85
	Pumpkin               // 86
	Netherrack            // 87
	SoulSand              // 88
	Glowstone             // 89
	_                     // 90
	LitPumpkin            // 91
	CakeBlock             // 92
	_                     // 93
	_                     // 94
	_                     // 95
	Trapdoor              // 96
	_                     // 97
	StoneBricks           // 98
	_                     // 99
	_                     // 100
	IronBar               // 101
	GlassPane             // 102
	MelonBlock            // 103
	PumpkinStem           // 104
	MelonStem             // 105
	Vine                  // 106
	FenceGate             // 107
	BrickStairs           // 108
	StoneBrickStairs      // 109
	Mycelium              // 110
	WaterLily             // 111
	NetherBricks          // 112
	NetherBrickFence      // 113
	NetherBricksStairs    // 114
	_                     // 115
	EnchantingTable       // 116
	BrewingStand          // 117
	_                     // 118
	_                     // 119
	EndPortal             // 120
	EndStone              // 121
	_                     // 122
	_                     // 123
	_                     // 124
	_                     // 125
	_                     // 126
	_                     // 127
	SandstoneStairs       // 128
	EmeraldOre            // 129
	_                     // 130
	_                     // 131
	_                     // 132
	EmeraldBlock          // 133
	SpruceWoodStairs      // 134
	BirchWoodStairs       // 135
	JungleWoodStairs      // 136
	_                     // 137
	_                     // 138
	CobbleWall            // 139
	FlowerPotBlock        // 140
	CarrotBlock           // 141
	PotatoBlock           // 142
	_                     // 143
	_                     // 144
	Anvil                 // 145
	TrappedChest          // 146
	_                     // 147
	_                     // 148
	_                     // 149
	_                     // 150
	_                     // 151
	RedstoneBlock         // 152
	_                     // 153
	_                     // 154
	QuartzBlock           // 155
	QuartzStairs          // 156
	DoubleWoodSlab        // 157
	WoodSlab              // 158
	StainedClay           // 159
	_                     // 160
	Leaves2               // 161
	Wood2                 // 162
	AcaciaWoodStairs      // 163
	DarkOakWoodStairs     // 164
	_                     // 165
	_                     // 166
	IronTrapdoor          // 167
	_                     // 168
	_                     // 169
	HayBale               // 170
	Carpet                // 171
	HardenedClay          // 172
	CoalBlock             // 173
	PackedIce             // 174
	DoublePlant           // 175
	_                     // 176
	_                     // 177
	_                     // 178
	_                     // 179
	_                     // 180
	_                     // 181
	_                     // 182
	FenceGateSpruce       // 183
	FenceGateBirch        // 184
	FenceGateJungle       // 185
	FenceGateDarkOak      // 186
	FenceGateAcacia       // 187
	_                     // 188
	_                     // 189
	_                     // 190
	_                     // 191
	_                     // 192
	_                     // 193
	_                     // 194
	_                     // 195
	_                     // 196
	_                     // 197
	GrassPath             // 198
	_                     // 199
	_                     // 200
	_                     // 201
	_                     // 202
	_                     // 203
	_                     // 204
	_                     // 205
	_                     // 206
	_                     // 207
	_                     // 208
	_                     // 209
	_                     // 210
	_                     // 211
	_                     // 212
	_                     // 213
	_                     // 214
	_                     // 215
	_                     // 216
	_                     // 217
	_                     // 218
	_                     // 219
	_                     // 220
	_                     // 221
	_                     // 222
	_                     // 223
	_                     // 224
	_                     // 225
	_                     // 226
	_                     // 227
	_                     // 228
	_                     // 229
	_                     // 230
	_                     // 231
	_                     // 232
	_                     // 233
	_                     // 234
	_                     // 235
	_                     // 236
	_                     // 237
	_                     // 238
	_                     // 239
	_                     // 240
	_                     // 241
	_                     // 242
	Podzol                // 243
	BeetrootBlock         // 244
	Stonecutter           // 245
	GlowingObsidian       // 246
	_                     // 247
	_                     // 248
	_                     // 249
	_                     // 250
	_                     // 251
	_                     // 252
	_                     // 253
	_                     // 254
	_                     // 255
	IronShovel            // 256
	IronPickaxe           // 257
	IronAxe               // 258
	FlintSteel            // 259
	Apple                 // 260
	Bow                   // 261
	Arrow                 // 262
	Coal                  // 263
	Diamond               // 264
	IronIngot             // 265
	GoldIngot             // 266
	IronSword             // 267
	WoodenSword           // 268
	WoodenShovel          // 269
	WoodenPickaxe         // 270
	WoodenAxe             // 271
	StoneSword            // 272
	StoneShovel           // 273
	StonePickaxe          // 274
	StoneAxe              // 275
	DiamondSword          // 276
	DiamondShovel         // 277
	DiamondPickaxe        // 278
	DiamondAxe            // 279
	Stick                 // 280
	Bowl                  // 281
	MushroomStew          // 282
	GoldSword             // 283
	GoldShovel            // 284
	GoldPickaxe           // 285
	GoldAxe               // 286
	String                // 287
	Feather               // 288
	Gunpowder             // 289
	WoodenHoe             // 290
	StoneHoe              // 291
	IronHoe               // 292
	DiamondHoe            // 293
	GoldHoe               // 294
	Seeds                 // 295
	Wheat                 // 296
	Bread                 // 297
	LeatherCap            // 298
	LeatherTunic          // 299
	LeatherPants          // 300
	LeatherBoots          // 301
	ChainHelmet           // 302
	ChainChestplate       // 303
	ChainLeggings         // 304
	ChainBoots            // 305
	IronHelmet            // 306
	IronChestplate        // 307
	IronLeggings          // 308
	IronBoots             // 309
	DiamondHelmet         // 310
	DiamondChestplate     // 311
	DiamondLeggings       // 312
	DiamondBoots          // 313
	GoldHelmet            // 314
	GoldChestplate        // 315
	GoldLeggings          // 316
	GoldBoots             // 317
	Flint                 // 318
	RawPorkchop           // 319
	CookedPorkchop        // 320
	Painting              // 321
	GoldenApple           // 322
	Sign                  // 323
	WoodenDoor            // 324
	Bucket                // 325
	_                     // 326
	_                     // 327
	Minecart              // 328
	_                     // 329
	IronDoor              // 330
	Redstone              // 331
	Snowball              // 332
	_                     // 333
	Leather               // 334
	_                     // 335
	Brick                 // 336
	Clay                  // 337
	Sugarcane             // 338
	Paper                 // 339
	Book                  // 340
	Slimeball             // 341
	_                     // 342
	_                     // 343
	Egg                   // 344
	Compass               // 345
	FishingRod            // 346
	Clock                 // 347
	GlowstoneDust         // 348
	RawFish               // 349
	CookedFish            // 350
	Dye                   // 351
	Bone                  // 352
	Sugar                 // 353
	Cake                  // 354
	Bed                   // 355
	_                     // 356
	Cookie                // 357
	_                     // 358
	Shears                // 359
	Melon                 // 360
	PumpkinSeeds          // 361
	MelonSeeds            // 362
	RawBeef               // 363
	Steak                 // 364
	RawChicken            // 365
	CookedChicken         // 366
	_                     // 367
	_                     // 368
	_                     // 369
	_                     // 370
	GoldNugget            // 371
	_                     // 372
	_                     // 373
	_                     // 374
	_                     // 375
	_                     // 376
	_                     // 377
	_                     // 378
	_                     // 379
	_                     // 380
	_                     // 381
	_                     // 382
	SpawnEgg              // 383
	_                     // 384
	_                     // 385
	_                     // 386
	_                     // 387
	Emerald               // 388
	_                     // 389
	FlowerPot             // 390
	Carrot                // 391
	Potato                // 392
	BakedPotato           // 393
	_                     // 394
	_                     // 395
	_                     // 396
	_                     // 397
	_                     // 398
	_                     // 399
	PumpkinPie            // 400
	_                     // 401
	_                     // 402
	_                     // 403
	_                     // 404
	NetherBrick           // 405
	Quartz                // 406
	_                     // 407
	_                     // 408
	_                     // 409
	_                     // 410
	_                     // 411
	_                     // 412
	_                     // 413
	_                     // 414
	_                     // 415
	_                     // 416
	_                     // 417
	_                     // 418
	_                     // 419
	_                     // 420
	_                     // 421
	_                     // 422
	_                     // 423
	_                     // 424
	_                     // 425
	_                     // 426
	_                     // 427
	_                     // 428
	_                     // 429
	_                     // 430
	_                     // 431
	_                     // 432
	_                     // 433
	_                     // 434
	_                     // 435
	_                     // 436
	_                     // 437
	_                     // 438
	_                     // 439
	_                     // 440
	_                     // 441
	_                     // 442
	_                     // 443
	_                     // 444
	_                     // 445
	_                     // 446
	_                     // 447
	_                     // 448
	_                     // 449
	_                     // 450
	_                     // 451
	_                     // 452
	_                     // 453
	_                     // 454
	_                     // 455
	Camera                // 456
	Beetroot              // 457
	BeetrootSeeds         // 458
	BeetrootSoup          // 459
)

// aliases
const (
	Rose                = Poppy              // 38
	JackOLantern        = LitPumpkin         // 91
	Workbench           = CraftingTable      // 58
	RedstoneDust        = Redstone           // 331
	BakedPotatoes       = BakedPotato        // 393
	Potatoes            = Potato             // 392
	StoneWall           = CobbleWall         // 139
	MelonSlice          = Melon              // 360
	LitRedstoneOre      = GlowingRedstoneOre // 74
	GoldenShovel        = GoldShovel         // 284
	WoodSlabs           = WoodSlab           // 158
	WheatSeeds          = Seeds              // 295
	EnchantmentTable    = EnchantingTable    // 116
	NetherQuartz        = Quartz             // 406
	EnchantTable        = EnchantingTable    // 116
	Planks              = Plank              // 5
	DarkOakWoodenStairs = DarkOakWoodStairs  // 164
	NetherBrickBlock    = NetherBricks       // 112
	WoodDoorBlock       = DoorBlock          // 64
	WoodenDoorBlock     = DoorBlock          // 64
	GoldenAxe           = GoldAxe            // 286
	OakWoodStairs       = WoodStairs         // 53
	MossyStone          = MossStone          // 48
	GlassPanel          = GlassPane          // 102
	CookedBeef          = Steak              // 364
	SnowLayer           = Snow               // 78
	SugarcaneBlock      = Reeds              // 83
	WoodenPlank         = Plank              // 5
	Trunk2              = Wood2              // 162
	GoldenSword         = GoldSword          // 283
	WoodenSlab          = WoodSlab           // 158
	WoodenStairs        = WoodStairs         // 53
	RedFlower           = Poppy              // 38
	AcaciaWoodenStairs  = AcaciaWoodStairs   // 163
	OakWoodenStairs     = WoodStairs         // 53
	FlintAndSteel       = FlintSteel         // 259
	Slabs               = Slab               // 44
	GlowstoneBlock      = Glowstone          // 89
	Leave2              = Leaves2            // 161
	DoubleWoodSlabs     = DoubleWoodSlab     // 157
	Carrots             = Carrot             // 391
	DoubleWoodenSlabs   = DoubleWoodSlab     // 157
	BeetrootSeed        = BeetrootSeeds      // 458
	SugarCane           = Sugarcane          // 338
	GoldenHoe           = GoldHoe            // 294
	CobblestoneWall     = CobbleWall         // 139
	StoneBrick          = StoneBricks        // 98
	LitFurnace          = BurningFurnace     // 62
	JungleWoodenStairs  = JungleWoodStairs   // 136
	SpruceWoodenStairs  = SpruceWoodStairs   // 134
	DeadBush            = Bush               // 32
	DoubleSlabs         = DoubleSlab         // 43
	LilyPad             = WaterLily          // 111
	Sticks              = Stick              // 280
	Log2                = Wood2              // 162
	Vines               = Vine               // 106
	WoodenPlanks        = Plank              // 5
	Cobble              = Cobblestone        // 4
	IronBars            = IronBar            // 101
	Saplings            = Sapling            // 6
	BricksBlock         = Bricks             // 45
	Leave               = Leaves             // 18
	Wood                = Log                // 17
	WoodenSlabs         = WoodSlab           // 158
	BirchWoodenStairs   = BirchWoodStairs    // 135
	Trunk               = Log                // 17
	DoubleWoodenSlab    = DoubleWoodSlab     // 157
	GoldenNugget        = GoldNugget         // 371
	SugarCanes          = Sugarcane          // 338
	CobblestoneStairs   = CobbleStairs       // 67
	StainedHardenedClay = StainedClay        // 159
	GoldenPickaxe       = GoldPickaxe        // 285
)

var idMap = map[string]ID{
	"Stone":              Stone,              // 1
	"Grass":              Grass,              // 2
	"Dirt":               Dirt,               // 3
	"Cobblestone":        Cobblestone,        // 4
	"Plank":              Plank,              // 5
	"Sapling":            Sapling,            // 6
	"Bedrock":            Bedrock,            // 7
	"Water":              Water,              // 8
	"StillWater":         StillWater,         // 9
	"Lava":               Lava,               // 10
	"StillLava":          StillLava,          // 11
	"Sand":               Sand,               // 12
	"Gravel":             Gravel,             // 13
	"GoldOre":            GoldOre,            // 14
	"IronOre":            IronOre,            // 15
	"CoalOre":            CoalOre,            // 16
	"Log":                Log,                // 17
	"Leaves":             Leaves,             // 18
	"Sponge":             Sponge,             // 19
	"Glass":              Glass,              // 20
	"LapisOre":           LapisOre,           // 21
	"LapisBlock":         LapisBlock,         // 22
	"Sandstone":          Sandstone,          // 24
	"BedBlock":           BedBlock,           // 26
	"Cobweb":             Cobweb,             // 30
	"TallGrass":          TallGrass,          // 31
	"Bush":               Bush,               // 32
	"Wool":               Wool,               // 35
	"Dandelion":          Dandelion,          // 37
	"Poppy":              Poppy,              // 38
	"BrownMushroom":      BrownMushroom,      // 39
	"RedMushroom":        RedMushroom,        // 40
	"GoldBlock":          GoldBlock,          // 41
	"IronBlock":          IronBlock,          // 42
	"DoubleSlab":         DoubleSlab,         // 43
	"Slab":               Slab,               // 44
	"Bricks":             Bricks,             // 45
	"Tnt":                Tnt,                // 46
	"Bookshelf":          Bookshelf,          // 47
	"MossStone":          MossStone,          // 48
	"Obsidian":           Obsidian,           // 49
	"Torch":              Torch,              // 50
	"Fire":               Fire,               // 51
	"MonsterSpawner":     MonsterSpawner,     // 52
	"WoodStairs":         WoodStairs,         // 53
	"Chest":              Chest,              // 54
	"DiamondOre":         DiamondOre,         // 56
	"DiamondBlock":       DiamondBlock,       // 57
	"CraftingTable":      CraftingTable,      // 58
	"WheatBlock":         WheatBlock,         // 59
	"Farmland":           Farmland,           // 60
	"Furnace":            Furnace,            // 61
	"BurningFurnace":     BurningFurnace,     // 62
	"SignPost":           SignPost,           // 63
	"DoorBlock":          DoorBlock,          // 64
	"Ladder":             Ladder,             // 65
	"CobbleStairs":       CobbleStairs,       // 67
	"WallSign":           WallSign,           // 68
	"IronDoorBlock":      IronDoorBlock,      // 71
	"RedstoneOre":        RedstoneOre,        // 73
	"GlowingRedstoneOre": GlowingRedstoneOre, // 74
	"Snow":               Snow,               // 78
	"Ice":                Ice,                // 79
	"SnowBlock":          SnowBlock,          // 80
	"Cactus":             Cactus,             // 81
	"ClayBlock":          ClayBlock,          // 82
	"Reeds":              Reeds,              // 83
	"Fence":              Fence,              // 85
	"Pumpkin":            Pumpkin,            // 86
	"Netherrack":         Netherrack,         // 87
	"SoulSand":           SoulSand,           // 88
	"Glowstone":          Glowstone,          // 89
	"LitPumpkin":         LitPumpkin,         // 91
	"CakeBlock":          CakeBlock,          // 92
	"Trapdoor":           Trapdoor,           // 96
	"StoneBricks":        StoneBricks,        // 98
	"IronBar":            IronBar,            // 101
	"GlassPane":          GlassPane,          // 102
	"MelonBlock":         MelonBlock,         // 103
	"PumpkinStem":        PumpkinStem,        // 104
	"MelonStem":          MelonStem,          // 105
	"Vine":               Vine,               // 106
	"FenceGate":          FenceGate,          // 107
	"BrickStairs":        BrickStairs,        // 108
	"StoneBrickStairs":   StoneBrickStairs,   // 109
	"Mycelium":           Mycelium,           // 110
	"WaterLily":          WaterLily,          // 111
	"NetherBricks":       NetherBricks,       // 112
	"NetherBrickFence":   NetherBrickFence,   // 113
	"NetherBricksStairs": NetherBricksStairs, // 114
	"EnchantingTable":    EnchantingTable,    // 116
	"BrewingStand":       BrewingStand,       // 117
	"EndPortal":          EndPortal,          // 120
	"EndStone":           EndStone,           // 121
	"SandstoneStairs":    SandstoneStairs,    // 128
	"EmeraldOre":         EmeraldOre,         // 129
	"EmeraldBlock":       EmeraldBlock,       // 133
	"SpruceWoodStairs":   SpruceWoodStairs,   // 134
	"BirchWoodStairs":    BirchWoodStairs,    // 135
	"JungleWoodStairs":   JungleWoodStairs,   // 136
	"CobbleWall":         CobbleWall,         // 139
	"FlowerPotBlock":     FlowerPotBlock,     // 140
	"CarrotBlock":        CarrotBlock,        // 141
	"PotatoBlock":        PotatoBlock,        // 142
	"Anvil":              Anvil,              // 145
	"TrappedChest":       TrappedChest,       // 146
	"RedstoneBlock":      RedstoneBlock,      // 152
	"QuartzBlock":        QuartzBlock,        // 155
	"QuartzStairs":       QuartzStairs,       // 156
	"DoubleWoodSlab":     DoubleWoodSlab,     // 157
	"WoodSlab":           WoodSlab,           // 158
	"StainedClay":        StainedClay,        // 159
	"Leaves2":            Leaves2,            // 161
	"Wood2":              Wood2,              // 162
	"AcaciaWoodStairs":   AcaciaWoodStairs,   // 163
	"DarkOakWoodStairs":  DarkOakWoodStairs,  // 164
	"IronTrapdoor":       IronTrapdoor,       // 167
	"HayBale":            HayBale,            // 170
	"Carpet":             Carpet,             // 171
	"HardenedClay":       HardenedClay,       // 172
	"CoalBlock":          CoalBlock,          // 173
	"PackedIce":          PackedIce,          // 174
	"DoublePlant":        DoublePlant,        // 175
	"FenceGateSpruce":    FenceGateSpruce,    // 183
	"FenceGateBirch":     FenceGateBirch,     // 184
	"FenceGateJungle":    FenceGateJungle,    // 185
	"FenceGateDarkOak":   FenceGateDarkOak,   // 186
	"FenceGateAcacia":    FenceGateAcacia,    // 187
	"GrassPath":          GrassPath,          // 198
	"Podzol":             Podzol,             // 243
	"BeetrootBlock":      BeetrootBlock,      // 244
	"Stonecutter":        Stonecutter,        // 245
	"GlowingObsidian":    GlowingObsidian,    // 246
	"IronShovel":         IronShovel,         // 256
	"IronPickaxe":        IronPickaxe,        // 257
	"IronAxe":            IronAxe,            // 258
	"FlintSteel":         FlintSteel,         // 259
	"Apple":              Apple,              // 260
	"Bow":                Bow,                // 261
	"Arrow":              Arrow,              // 262
	"Coal":               Coal,               // 263
	"Diamond":            Diamond,            // 264
	"IronIngot":          IronIngot,          // 265
	"GoldIngot":          GoldIngot,          // 266
	"IronSword":          IronSword,          // 267
	"WoodenSword":        WoodenSword,        // 268
	"WoodenShovel":       WoodenShovel,       // 269
	"WoodenPickaxe":      WoodenPickaxe,      // 270
	"WoodenAxe":          WoodenAxe,          // 271
	"StoneSword":         StoneSword,         // 272
	"StoneShovel":        StoneShovel,        // 273
	"StonePickaxe":       StonePickaxe,       // 274
	"StoneAxe":           StoneAxe,           // 275
	"DiamondSword":       DiamondSword,       // 276
	"DiamondShovel":      DiamondShovel,      // 277
	"DiamondPickaxe":     DiamondPickaxe,     // 278
	"DiamondAxe":         DiamondAxe,         // 279
	"Stick":              Stick,              // 280
	"Bowl":               Bowl,               // 281
	"MushroomStew":       MushroomStew,       // 282
	"GoldSword":          GoldSword,          // 283
	"GoldShovel":         GoldShovel,         // 284
	"GoldPickaxe":        GoldPickaxe,        // 285
	"GoldAxe":            GoldAxe,            // 286
	"String":             String,             // 287
	"Feather":            Feather,            // 288
	"Gunpowder":          Gunpowder,          // 289
	"WoodenHoe":          WoodenHoe,          // 290
	"StoneHoe":           StoneHoe,           // 291
	"IronHoe":            IronHoe,            // 292
	"DiamondHoe":         DiamondHoe,         // 293
	"GoldHoe":            GoldHoe,            // 294
	"Seeds":              Seeds,              // 295
	"Wheat":              Wheat,              // 296
	"Bread":              Bread,              // 297
	"LeatherCap":         LeatherCap,         // 298
	"LeatherTunic":       LeatherTunic,       // 299
	"LeatherPants":       LeatherPants,       // 300
	"LeatherBoots":       LeatherBoots,       // 301
	"ChainHelmet":        ChainHelmet,        // 302
	"ChainChestplate":    ChainChestplate,    // 303
	"ChainLeggings":      ChainLeggings,      // 304
	"ChainBoots":         ChainBoots,         // 305
	"IronHelmet":         IronHelmet,         // 306
	"IronChestplate":     IronChestplate,     // 307
	"IronLeggings":       IronLeggings,       // 308
	"IronBoots":          IronBoots,          // 309
	"DiamondHelmet":      DiamondHelmet,      // 310
	"DiamondChestplate":  DiamondChestplate,  // 311
	"DiamondLeggings":    DiamondLeggings,    // 312
	"DiamondBoots":       DiamondBoots,       // 313
	"GoldHelmet":         GoldHelmet,         // 314
	"GoldChestplate":     GoldChestplate,     // 315
	"GoldLeggings":       GoldLeggings,       // 316
	"GoldBoots":          GoldBoots,          // 317
	"Flint":              Flint,              // 318
	"RawPorkchop":        RawPorkchop,        // 319
	"CookedPorkchop":     CookedPorkchop,     // 320
	"Painting":           Painting,           // 321
	"GoldenApple":        GoldenApple,        // 322
	"Sign":               Sign,               // 323
	"WoodenDoor":         WoodenDoor,         // 324
	"Bucket":             Bucket,             // 325
	"Minecart":           Minecart,           // 328
	"IronDoor":           IronDoor,           // 330
	"Redstone":           Redstone,           // 331
	"Snowball":           Snowball,           // 332
	"Leather":            Leather,            // 334
	"Brick":              Brick,              // 336
	"Clay":               Clay,               // 337
	"Sugarcane":          Sugarcane,          // 338
	"Paper":              Paper,              // 339
	"Book":               Book,               // 340
	"Slimeball":          Slimeball,          // 341
	"Egg":                Egg,                // 344
	"Compass":            Compass,            // 345
	"FishingRod":         FishingRod,         // 346
	"Clock":              Clock,              // 347
	"GlowstoneDust":      GlowstoneDust,      // 348
	"RawFish":            RawFish,            // 349
	"CookedFish":         CookedFish,         // 350
	"Dye":                Dye,                // 351
	"Bone":               Bone,               // 352
	"Sugar":              Sugar,              // 353
	"Cake":               Cake,               // 354
	"Bed":                Bed,                // 355
	"Cookie":             Cookie,             // 357
	"Shears":             Shears,             // 359
	"Melon":              Melon,              // 360
	"PumpkinSeeds":       PumpkinSeeds,       // 361
	"MelonSeeds":         MelonSeeds,         // 362
	"RawBeef":            RawBeef,            // 363
	"Steak":              Steak,              // 364
	"RawChicken":         RawChicken,         // 365
	"CookedChicken":      CookedChicken,      // 366
	"GoldNugget":         GoldNugget,         // 371
	"SpawnEgg":           SpawnEgg,           // 383
	"Emerald":            Emerald,            // 388
	"FlowerPot":          FlowerPot,          // 390
	"Carrot":             Carrot,             // 391
	"Potato":             Potato,             // 392
	"BakedPotato":        BakedPotato,        // 393
	"PumpkinPie":         PumpkinPie,         // 400
	"NetherBrick":        NetherBrick,        // 405
	"Quartz":             Quartz,             // 406
	"Camera":             Camera,             // 456
	"Beetroot":           Beetroot,           // 457
	"BeetrootSeeds":      BeetrootSeeds,      // 458
	"BeetrootSoup":       BeetrootSoup,       // 459

	//aliases

	"Rose":                Rose,                // 38
	"JackOLantern":        JackOLantern,        // 91
	"Workbench":           Workbench,           // 58
	"RedstoneDust":        RedstoneDust,        // 331
	"BakedPotatoes":       BakedPotatoes,       // 393
	"Potatoes":            Potatoes,            // 392
	"StoneWall":           StoneWall,           // 139
	"MelonSlice":          MelonSlice,          // 360
	"LitRedstoneOre":      LitRedstoneOre,      // 74
	"GoldenShovel":        GoldenShovel,        // 284
	"WoodSlabs":           WoodSlabs,           // 158
	"WheatSeeds":          WheatSeeds,          // 295
	"EnchantmentTable":    EnchantmentTable,    // 116
	"NetherQuartz":        NetherQuartz,        // 406
	"EnchantTable":        EnchantTable,        // 116
	"Planks":              Planks,              // 5
	"DarkOakWoodenStairs": DarkOakWoodenStairs, // 164
	"NetherBrickBlock":    NetherBrickBlock,    // 112
	"WoodDoorBlock":       WoodDoorBlock,       // 64
	"WoodenDoorBlock":     WoodenDoorBlock,     // 64
	"GoldenAxe":           GoldenAxe,           // 286
	"OakWoodStairs":       OakWoodStairs,       // 53
	"MossyStone":          MossyStone,          // 48
	"GlassPanel":          GlassPanel,          // 102
	"CookedBeef":          CookedBeef,          // 364
	"SnowLayer":           SnowLayer,           // 78
	"SugarcaneBlock":      SugarcaneBlock,      // 83
	"WoodenPlank":         WoodenPlank,         // 5
	"Trunk2":              Trunk2,              // 162
	"GoldenSword":         GoldenSword,         // 283
	"WoodenSlab":          WoodenSlab,          // 158
	"WoodenStairs":        WoodenStairs,        // 53
	"RedFlower":           RedFlower,           // 38
	"AcaciaWoodenStairs":  AcaciaWoodenStairs,  // 163
	"OakWoodenStairs":     OakWoodenStairs,     // 53
	"FlintAndSteel":       FlintAndSteel,       // 259
	"Slabs":               Slabs,               // 44
	"GlowstoneBlock":      GlowstoneBlock,      // 89
	"Leave2":              Leave2,              // 161
	"DoubleWoodSlabs":     DoubleWoodSlabs,     // 157
	"Carrots":             Carrots,             // 391
	"DoubleWoodenSlabs":   DoubleWoodenSlabs,   // 157
	"BeetrootSeed":        BeetrootSeed,        // 458
	"SugarCane":           SugarCane,           // 338
	"GoldenHoe":           GoldenHoe,           // 294
	"CobblestoneWall":     CobblestoneWall,     // 139
	"StoneBrick":          StoneBrick,          // 98
	"LitFurnace":          LitFurnace,          // 62
	"JungleWoodenStairs":  JungleWoodenStairs,  // 136
	"SpruceWoodenStairs":  SpruceWoodenStairs,  // 134
	"DeadBush":            DeadBush,            // 32
	"DoubleSlabs":         DoubleSlabs,         // 43
	"LilyPad":             LilyPad,             // 111
	"Sticks":              Sticks,              // 280
	"Log2":                Log2,                // 162
	"Vines":               Vines,               // 106
	"WoodenPlanks":        WoodenPlanks,        // 5
	"Cobble":              Cobble,              // 4
	"IronBars":            IronBars,            // 101
	"Saplings":            Saplings,            // 6
	"BricksBlock":         BricksBlock,         // 45
	"Leave":               Leave,               // 18
	"Wood":                Wood,                // 17
	"WoodenSlabs":         WoodenSlabs,         // 158
	"BirchWoodenStairs":   BirchWoodenStairs,   // 135
	"Trunk":               Trunk,               // 17
	"DoubleWoodenSlab":    DoubleWoodenSlab,    // 157
	"GoldenNugget":        GoldenNugget,        // 371
	"SugarCanes":          SugarCanes,          // 338
	"CobblestoneStairs":   CobblestoneStairs,   // 67
	"StainedHardenedClay": StainedHardenedClay, // 159
	"GoldenPickaxe":       GoldenPickaxe,       // 285
}

var nameMap = map[ID]string{
	Stone:              "Stone",              // 1
	Grass:              "Grass",              // 2
	Dirt:               "Dirt",               // 3
	Cobblestone:        "Cobblestone",        // 4
	Plank:              "Plank",              // 5
	Sapling:            "Sapling",            // 6
	Bedrock:            "Bedrock",            // 7
	Water:              "Water",              // 8
	StillWater:         "StillWater",         // 9
	Lava:               "Lava",               // 10
	StillLava:          "StillLava",          // 11
	Sand:               "Sand",               // 12
	Gravel:             "Gravel",             // 13
	GoldOre:            "GoldOre",            // 14
	IronOre:            "IronOre",            // 15
	CoalOre:            "CoalOre",            // 16
	Log:                "Log",                // 17
	Leaves:             "Leaves",             // 18
	Sponge:             "Sponge",             // 19
	Glass:              "Glass",              // 20
	LapisOre:           "LapisOre",           // 21
	LapisBlock:         "LapisBlock",         // 22
	Sandstone:          "Sandstone",          // 24
	BedBlock:           "BedBlock",           // 26
	Cobweb:             "Cobweb",             // 30
	TallGrass:          "TallGrass",          // 31
	Bush:               "Bush",               // 32
	Wool:               "Wool",               // 35
	Dandelion:          "Dandelion",          // 37
	Poppy:              "Poppy",              // 38
	BrownMushroom:      "BrownMushroom",      // 39
	RedMushroom:        "RedMushroom",        // 40
	GoldBlock:          "GoldBlock",          // 41
	IronBlock:          "IronBlock",          // 42
	DoubleSlab:         "DoubleSlab",         // 43
	Slab:               "Slab",               // 44
	Bricks:             "Bricks",             // 45
	Tnt:                "Tnt",                // 46
	Bookshelf:          "Bookshelf",          // 47
	MossStone:          "MossStone",          // 48
	Obsidian:           "Obsidian",           // 49
	Torch:              "Torch",              // 50
	Fire:               "Fire",               // 51
	MonsterSpawner:     "MonsterSpawner",     // 52
	WoodStairs:         "WoodStairs",         // 53
	Chest:              "Chest",              // 54
	DiamondOre:         "DiamondOre",         // 56
	DiamondBlock:       "DiamondBlock",       // 57
	CraftingTable:      "CraftingTable",      // 58
	WheatBlock:         "WheatBlock",         // 59
	Farmland:           "Farmland",           // 60
	Furnace:            "Furnace",            // 61
	BurningFurnace:     "BurningFurnace",     // 62
	SignPost:           "SignPost",           // 63
	DoorBlock:          "DoorBlock",          // 64
	Ladder:             "Ladder",             // 65
	CobbleStairs:       "CobbleStairs",       // 67
	WallSign:           "WallSign",           // 68
	IronDoorBlock:      "IronDoorBlock",      // 71
	RedstoneOre:        "RedstoneOre",        // 73
	GlowingRedstoneOre: "GlowingRedstoneOre", // 74
	Snow:               "Snow",               // 78
	Ice:                "Ice",                // 79
	SnowBlock:          "SnowBlock",          // 80
	Cactus:             "Cactus",             // 81
	ClayBlock:          "ClayBlock",          // 82
	Reeds:              "Reeds",              // 83
	Fence:              "Fence",              // 85
	Pumpkin:            "Pumpkin",            // 86
	Netherrack:         "Netherrack",         // 87
	SoulSand:           "SoulSand",           // 88
	Glowstone:          "Glowstone",          // 89
	LitPumpkin:         "LitPumpkin",         // 91
	CakeBlock:          "CakeBlock",          // 92
	Trapdoor:           "Trapdoor",           // 96
	StoneBricks:        "StoneBricks",        // 98
	IronBar:            "IronBar",            // 101
	GlassPane:          "GlassPane",          // 102
	MelonBlock:         "MelonBlock",         // 103
	PumpkinStem:        "PumpkinStem",        // 104
	MelonStem:          "MelonStem",          // 105
	Vine:               "Vine",               // 106
	FenceGate:          "FenceGate",          // 107
	BrickStairs:        "BrickStairs",        // 108
	StoneBrickStairs:   "StoneBrickStairs",   // 109
	Mycelium:           "Mycelium",           // 110
	WaterLily:          "WaterLily",          // 111
	NetherBricks:       "NetherBricks",       // 112
	NetherBrickFence:   "NetherBrickFence",   // 113
	NetherBricksStairs: "NetherBricksStairs", // 114
	EnchantingTable:    "EnchantingTable",    // 116
	BrewingStand:       "BrewingStand",       // 117
	EndPortal:          "EndPortal",          // 120
	EndStone:           "EndStone",           // 121
	SandstoneStairs:    "SandstoneStairs",    // 128
	EmeraldOre:         "EmeraldOre",         // 129
	EmeraldBlock:       "EmeraldBlock",       // 133
	SpruceWoodStairs:   "SpruceWoodStairs",   // 134
	BirchWoodStairs:    "BirchWoodStairs",    // 135
	JungleWoodStairs:   "JungleWoodStairs",   // 136
	CobbleWall:         "CobbleWall",         // 139
	FlowerPotBlock:     "FlowerPotBlock",     // 140
	CarrotBlock:        "CarrotBlock",        // 141
	PotatoBlock:        "PotatoBlock",        // 142
	Anvil:              "Anvil",              // 145
	TrappedChest:       "TrappedChest",       // 146
	RedstoneBlock:      "RedstoneBlock",      // 152
	QuartzBlock:        "QuartzBlock",        // 155
	QuartzStairs:       "QuartzStairs",       // 156
	DoubleWoodSlab:     "DoubleWoodSlab",     // 157
	WoodSlab:           "WoodSlab",           // 158
	StainedClay:        "StainedClay",        // 159
	Leaves2:            "Leaves2",            // 161
	Wood2:              "Wood2",              // 162
	AcaciaWoodStairs:   "AcaciaWoodStairs",   // 163
	DarkOakWoodStairs:  "DarkOakWoodStairs",  // 164
	IronTrapdoor:       "IronTrapdoor",       // 167
	HayBale:            "HayBale",            // 170
	Carpet:             "Carpet",             // 171
	HardenedClay:       "HardenedClay",       // 172
	CoalBlock:          "CoalBlock",          // 173
	PackedIce:          "PackedIce",          // 174
	DoublePlant:        "DoublePlant",        // 175
	FenceGateSpruce:    "FenceGateSpruce",    // 183
	FenceGateBirch:     "FenceGateBirch",     // 184
	FenceGateJungle:    "FenceGateJungle",    // 185
	FenceGateDarkOak:   "FenceGateDarkOak",   // 186
	FenceGateAcacia:    "FenceGateAcacia",    // 187
	GrassPath:          "GrassPath",          // 198
	Podzol:             "Podzol",             // 243
	BeetrootBlock:      "BeetrootBlock",      // 244
	Stonecutter:        "Stonecutter",        // 245
	GlowingObsidian:    "GlowingObsidian",    // 246
	IronShovel:         "IronShovel",         // 256
	IronPickaxe:        "IronPickaxe",        // 257
	IronAxe:            "IronAxe",            // 258
	FlintSteel:         "FlintSteel",         // 259
	Apple:              "Apple",              // 260
	Bow:                "Bow",                // 261
	Arrow:              "Arrow",              // 262
	Coal:               "Coal",               // 263
	Diamond:            "Diamond",            // 264
	IronIngot:          "IronIngot",          // 265
	GoldIngot:          "GoldIngot",          // 266
	IronSword:          "IronSword",          // 267
	WoodenSword:        "WoodenSword",        // 268
	WoodenShovel:       "WoodenShovel",       // 269
	WoodenPickaxe:      "WoodenPickaxe",      // 270
	WoodenAxe:          "WoodenAxe",          // 271
	StoneSword:         "StoneSword",         // 272
	StoneShovel:        "StoneShovel",        // 273
	StonePickaxe:       "StonePickaxe",       // 274
	StoneAxe:           "StoneAxe",           // 275
	DiamondSword:       "DiamondSword",       // 276
	DiamondShovel:      "DiamondShovel",      // 277
	DiamondPickaxe:     "DiamondPickaxe",     // 278
	DiamondAxe:         "DiamondAxe",         // 279
	Stick:              "Stick",              // 280
	Bowl:               "Bowl",               // 281
	MushroomStew:       "MushroomStew",       // 282
	GoldSword:          "GoldSword",          // 283
	GoldShovel:         "GoldShovel",         // 284
	GoldPickaxe:        "GoldPickaxe",        // 285
	GoldAxe:            "GoldAxe",            // 286
	String:             "String",             // 287
	Feather:            "Feather",            // 288
	Gunpowder:          "Gunpowder",          // 289
	WoodenHoe:          "WoodenHoe",          // 290
	StoneHoe:           "StoneHoe",           // 291
	IronHoe:            "IronHoe",            // 292
	DiamondHoe:         "DiamondHoe",         // 293
	GoldHoe:            "GoldHoe",            // 294
	Seeds:              "Seeds",              // 295
	Wheat:              "Wheat",              // 296
	Bread:              "Bread",              // 297
	LeatherCap:         "LeatherCap",         // 298
	LeatherTunic:       "LeatherTunic",       // 299
	LeatherPants:       "LeatherPants",       // 300
	LeatherBoots:       "LeatherBoots",       // 301
	ChainHelmet:        "ChainHelmet",        // 302
	ChainChestplate:    "ChainChestplate",    // 303
	ChainLeggings:      "ChainLeggings",      // 304
	ChainBoots:         "ChainBoots",         // 305
	IronHelmet:         "IronHelmet",         // 306
	IronChestplate:     "IronChestplate",     // 307
	IronLeggings:       "IronLeggings",       // 308
	IronBoots:          "IronBoots",          // 309
	DiamondHelmet:      "DiamondHelmet",      // 310
	DiamondChestplate:  "DiamondChestplate",  // 311
	DiamondLeggings:    "DiamondLeggings",    // 312
	DiamondBoots:       "DiamondBoots",       // 313
	GoldHelmet:         "GoldHelmet",         // 314
	GoldChestplate:     "GoldChestplate",     // 315
	GoldLeggings:       "GoldLeggings",       // 316
	GoldBoots:          "GoldBoots",          // 317
	Flint:              "Flint",              // 318
	RawPorkchop:        "RawPorkchop",        // 319
	CookedPorkchop:     "CookedPorkchop",     // 320
	Painting:           "Painting",           // 321
	GoldenApple:        "GoldenApple",        // 322
	Sign:               "Sign",               // 323
	WoodenDoor:         "WoodenDoor",         // 324
	Bucket:             "Bucket",             // 325
	Minecart:           "Minecart",           // 328
	IronDoor:           "IronDoor",           // 330
	Redstone:           "Redstone",           // 331
	Snowball:           "Snowball",           // 332
	Leather:            "Leather",            // 334
	Brick:              "Brick",              // 336
	Clay:               "Clay",               // 337
	Sugarcane:          "Sugarcane",          // 338
	Paper:              "Paper",              // 339
	Book:               "Book",               // 340
	Slimeball:          "Slimeball",          // 341
	Egg:                "Egg",                // 344
	Compass:            "Compass",            // 345
	FishingRod:         "FishingRod",         // 346
	Clock:              "Clock",              // 347
	GlowstoneDust:      "GlowstoneDust",      // 348
	RawFish:            "RawFish",            // 349
	CookedFish:         "CookedFish",         // 350
	Dye:                "Dye",                // 351
	Bone:               "Bone",               // 352
	Sugar:              "Sugar",              // 353
	Cake:               "Cake",               // 354
	Bed:                "Bed",                // 355
	Cookie:             "Cookie",             // 357
	Shears:             "Shears",             // 359
	Melon:              "Melon",              // 360
	PumpkinSeeds:       "PumpkinSeeds",       // 361
	MelonSeeds:         "MelonSeeds",         // 362
	RawBeef:            "RawBeef",            // 363
	Steak:              "Steak",              // 364
	RawChicken:         "RawChicken",         // 365
	CookedChicken:      "CookedChicken",      // 366
	GoldNugget:         "GoldNugget",         // 371
	SpawnEgg:           "SpawnEgg",           // 383
	Emerald:            "Emerald",            // 388
	FlowerPot:          "FlowerPot",          // 390
	Carrot:             "Carrot",             // 391
	Potato:             "Potato",             // 392
	BakedPotato:        "BakedPotato",        // 393
	PumpkinPie:         "PumpkinPie",         // 400
	NetherBrick:        "NetherBrick",        // 405
	Quartz:             "Quartz",             // 406
	Camera:             "Camera",             // 456
	Beetroot:           "Beetroot",           // 457
	BeetrootSeeds:      "BeetrootSeeds",      // 458
	BeetrootSoup:       "BeetrootSoup",       // 459
}

var updateMap = map[byte]struct{}{
	Torch.Block(): {},
}

// StringID returns item ID with given name.
// If there's no such item, returns -1(65535).
func StringID(name string) ID {
	if id, ok := idMap[name]; ok {
		return id
	}
	return 65535
}

// CreativeItems is a list of inventory items for creative mode players.
var CreativeItems = []Item{
	{ID: 4, Meta: 0},
	{ID: 98, Meta: 0},
	{ID: 98, Meta: 1},
	{ID: 98, Meta: 2},
	{ID: 98, Meta: 3},
	{ID: 48, Meta: 0},
	{ID: 5, Meta: 0},
	{ID: 5, Meta: 1},
	{ID: 5, Meta: 2},
	{ID: 5, Meta: 3},
	{ID: 5, Meta: 4},
	{ID: 5, Meta: 5},
	{ID: 45, Meta: 0},
	{ID: 1, Meta: 0},
	{ID: 1, Meta: 1},
	{ID: 1, Meta: 2},
	{ID: 1, Meta: 3},
	{ID: 1, Meta: 4},
	{ID: 1, Meta: 5},
	{ID: 1, Meta: 6},
	{ID: 3, Meta: 0},
	{ID: 243, Meta: 0},
	{ID: 2, Meta: 0},
	{ID: 110, Meta: 0},
	{ID: 82, Meta: 0},
	{ID: 172, Meta: 0},
	{ID: 159, Meta: 0},
	{ID: 159, Meta: 1},
	{ID: 159, Meta: 2},
	{ID: 159, Meta: 3},
	{ID: 159, Meta: 4},
	{ID: 159, Meta: 5},
	{ID: 159, Meta: 6},
	{ID: 159, Meta: 7},
	{ID: 159, Meta: 8},
	{ID: 159, Meta: 9},
	{ID: 159, Meta: 10},
	{ID: 159, Meta: 11},
	{ID: 159, Meta: 12},
	{ID: 159, Meta: 13},
	{ID: 159, Meta: 14},
	{ID: 159, Meta: 15},
	{ID: 24, Meta: 0},
	{ID: 24, Meta: 1},
	{ID: 24, Meta: 2},
	{ID: 12, Meta: 0},
	{ID: 12, Meta: 1},
	{ID: 13, Meta: 0},
	{ID: 17, Meta: 0},
	{ID: 17, Meta: 1},
	{ID: 17, Meta: 2},
	{ID: 17, Meta: 3},
	{ID: 162, Meta: 0},
	{ID: 162, Meta: 1},
	{ID: 112, Meta: 0},
	{ID: 87, Meta: 0},
	{ID: 88, Meta: 0},
	{ID: 7, Meta: 0},
	{ID: 67, Meta: 0},
	{ID: 53, Meta: 0},
	{ID: 134, Meta: 0},
	{ID: 135, Meta: 0},
	{ID: 136, Meta: 0},
	{ID: 163, Meta: 0},
	{ID: 164, Meta: 0},
	{ID: 108, Meta: 0},
	{ID: 128, Meta: 0},
	{ID: 109, Meta: 0},
	{ID: 114, Meta: 0},
	{ID: 156, Meta: 0},
	{ID: 44, Meta: 0},
	{ID: 44, Meta: 1},
	{ID: 158, Meta: 0},
	{ID: 158, Meta: 1},
	{ID: 158, Meta: 2},
	{ID: 158, Meta: 3},
	{ID: 158, Meta: 4},
	{ID: 158, Meta: 5},
	{ID: 44, Meta: 3},
	{ID: 44, Meta: 4},
	{ID: 44, Meta: 5},
	{ID: 44, Meta: 6},
	{ID: 44, Meta: 7},
	{ID: 155, Meta: 0},
	{ID: 155, Meta: 1},
	{ID: 155, Meta: 2},
	{ID: 16, Meta: 0},
	{ID: 15, Meta: 0},
	{ID: 14, Meta: 0},
	{ID: 56, Meta: 0},
	{ID: 21, Meta: 0},
	{ID: 73, Meta: 0},
	{ID: 129, Meta: 0},
	{ID: 49, Meta: 0},
	{ID: 79, Meta: 0},
	{ID: 174, Meta: 0},
	{ID: 80, Meta: 0},
	{ID: 121, Meta: 0},
	{ID: 139, Meta: 0},
	{ID: 139, Meta: 1},
	{ID: 111, Meta: 0},
	{ID: 41, Meta: 0},
	{ID: 42, Meta: 0},
	{ID: 57, Meta: 0},
	{ID: 22, Meta: 0},
	{ID: 173, Meta: 0},
	{ID: 133, Meta: 0},
	{ID: 152, Meta: 0},
	{ID: 78, Meta: 0},
	{ID: 20, Meta: 0},
	{ID: 89, Meta: 0},
	{ID: 106, Meta: 0},
	{ID: 65, Meta: 0},
	{ID: 19, Meta: 0},
	{ID: 102, Meta: 0},
	{ID: 324, Meta: 0},
	{ID: 324, Meta: 0},
	{ID: 324, Meta: 0},
	{ID: 324, Meta: 0},
	{ID: 324, Meta: 0},
	{ID: 324, Meta: 0},
	{ID: 330, Meta: 0},
	{ID: 96, Meta: 0},
	{ID: 167, Meta: 0},
	{ID: 85, Meta: 0},
	{ID: 85, Meta: 1},
	{ID: 85, Meta: 2},
	{ID: 85, Meta: 3},
	{ID: 85, Meta: 4},
	{ID: 85, Meta: 5},
	{ID: 113, Meta: 0},
	{ID: 107, Meta: 0},
	{ID: 183, Meta: 0},
	{ID: 184, Meta: 0},
	{ID: 185, Meta: 0},
	{ID: 187, Meta: 0},
	{ID: 186, Meta: 0},
	{ID: 101, Meta: 0},
	{ID: 355, Meta: 0},
	{ID: 47, Meta: 0},
	{ID: 321, Meta: 0},
	{ID: 58, Meta: 0},
	{ID: 245, Meta: 0},
	{ID: 54, Meta: 0},
	{ID: 54, Meta: 0},
	{ID: 61, Meta: 0},
	{ID: 379, Meta: 0},
	{ID: 120, Meta: 0},
	{ID: 145, Meta: 0},
	{ID: 145, Meta: 4},
	{ID: 145, Meta: 8},
	{ID: 37, Meta: 0},
	{ID: 38, Meta: 0},
	{ID: 38, Meta: 1},
	{ID: 38, Meta: 2},
	{ID: 38, Meta: 3},
	{ID: 38, Meta: 4},
	{ID: 38, Meta: 5},
	{ID: 38, Meta: 6},
	{ID: 38, Meta: 7},
	{ID: 38, Meta: 8},
	{ID: 39, Meta: 0},
	{ID: 40, Meta: 0},
	{ID: 81, Meta: 0},
	{ID: 103, Meta: 0},
	{ID: 86, Meta: 0},
	{ID: 91, Meta: 0},
	{ID: 30, Meta: 0},
	{ID: 170, Meta: 0},
	{ID: 31, Meta: 1},
	{ID: 31, Meta: 2},
	{ID: 32, Meta: 0},
	{ID: 6, Meta: 0},
	{ID: 6, Meta: 1},
	{ID: 6, Meta: 2},
	{ID: 6, Meta: 3},
	{ID: 6, Meta: 4},
	{ID: 6, Meta: 5},
	{ID: 18, Meta: 0},
	{ID: 18, Meta: 1},
	{ID: 18, Meta: 2},
	{ID: 18, Meta: 3},
	{ID: 161, Meta: 0},
	{ID: 161, Meta: 1},
	{ID: 354, Meta: 0},
	{ID: 323, Meta: 0},
	{ID: 390, Meta: 0},
	{ID: 52, Meta: 0},
	{ID: 116, Meta: 0},
	{ID: 35, Meta: 0},
	{ID: 35, Meta: 7},
	{ID: 35, Meta: 6},
	{ID: 35, Meta: 5},
	{ID: 35, Meta: 4},
	{ID: 35, Meta: 3},
	{ID: 35, Meta: 2},
	{ID: 35, Meta: 1},
	{ID: 35, Meta: 15},
	{ID: 35, Meta: 14},
	{ID: 35, Meta: 13},
	{ID: 35, Meta: 12},
	{ID: 35, Meta: 11},
	{ID: 35, Meta: 10},
	{ID: 35, Meta: 9},
	{ID: 35, Meta: 8},
	{ID: 171, Meta: 0},
	{ID: 171, Meta: 7},
	{ID: 171, Meta: 6},
	{ID: 171, Meta: 5},
	{ID: 171, Meta: 4},
	{ID: 171, Meta: 3},
	{ID: 171, Meta: 2},
	{ID: 171, Meta: 1},
	{ID: 171, Meta: 15},
	{ID: 171, Meta: 14},
	{ID: 171, Meta: 13},
	{ID: 171, Meta: 12},
	{ID: 171, Meta: 11},
	{ID: 171, Meta: 10},
	{ID: 171, Meta: 9},
	{ID: 171, Meta: 8},
	{ID: 139, Meta: 0},
	{ID: 139, Meta: 1},
	{ID: 111, Meta: 0},
	{ID: 41, Meta: 0},
	{ID: 42, Meta: 0},
	{ID: 57, Meta: 0},
	{ID: 22, Meta: 0},
	{ID: 173, Meta: 0},
	{ID: 133, Meta: 0},
	{ID: 152, Meta: 0},
	{ID: 78, Meta: 0},
	{ID: 20, Meta: 0},
	{ID: 89, Meta: 0},
	{ID: 106, Meta: 0},
	{ID: 65, Meta: 0},
	{ID: 19, Meta: 0},
	{ID: 102, Meta: 0},
	{ID: 324, Meta: 0},
	{ID: 324, Meta: 0},
	{ID: 324, Meta: 0},
	{ID: 324, Meta: 0},
	{ID: 324, Meta: 0},
	{ID: 324, Meta: 0},
	{ID: 330, Meta: 0},
	{ID: 96, Meta: 0},
	{ID: 167, Meta: 0},
	{ID: 85, Meta: 0},
	{ID: 85, Meta: 1},
	{ID: 85, Meta: 2},
	{ID: 85, Meta: 3},
	{ID: 85, Meta: 4},
	{ID: 85, Meta: 5},
	{ID: 113, Meta: 0},
	{ID: 107, Meta: 0},
	{ID: 183, Meta: 0},
	{ID: 184, Meta: 0},
	{ID: 185, Meta: 0},
	{ID: 187, Meta: 0},
	{ID: 186, Meta: 0},
	{ID: 101, Meta: 0},
	{ID: 355, Meta: 0},
	{ID: 47, Meta: 0},
	{ID: 321, Meta: 0},
	{ID: 58, Meta: 0},
	{ID: 245, Meta: 0},
	{ID: 54, Meta: 0},
	{ID: 54, Meta: 0},
	{ID: 61, Meta: 0},
	{ID: 379, Meta: 0},
	{ID: 120, Meta: 0},
	{ID: 145, Meta: 0},
	{ID: 145, Meta: 4},
	{ID: 145, Meta: 8},
	{ID: 37, Meta: 0},
	{ID: 38, Meta: 0},
	{ID: 38, Meta: 1},
	{ID: 38, Meta: 2},
	{ID: 38, Meta: 3},
	{ID: 38, Meta: 4},
	{ID: 38, Meta: 5},
	{ID: 38, Meta: 6},
	{ID: 38, Meta: 7},
	{ID: 38, Meta: 8},
	{ID: 39, Meta: 0},
	{ID: 40, Meta: 0},
	{ID: 81, Meta: 0},
	{ID: 103, Meta: 0},
	{ID: 86, Meta: 0},
	{ID: 91, Meta: 0},
	{ID: 30, Meta: 0},
	{ID: 170, Meta: 0},
	{ID: 31, Meta: 1},
	{ID: 31, Meta: 2},
	{ID: 32, Meta: 0},
	{ID: 6, Meta: 0},
	{ID: 6, Meta: 1},
	{ID: 6, Meta: 2},
	{ID: 6, Meta: 3},
	{ID: 6, Meta: 4},
	{ID: 6, Meta: 5},
	{ID: 18, Meta: 0},
	{ID: 18, Meta: 1},
	{ID: 18, Meta: 2},
	{ID: 18, Meta: 3},
	{ID: 161, Meta: 0},
	{ID: 161, Meta: 1},
	{ID: 354, Meta: 0},
	{ID: 323, Meta: 0},
	{ID: 390, Meta: 0},
	{ID: 52, Meta: 0},
	{ID: 116, Meta: 0},
	{ID: 35, Meta: 0},
	{ID: 35, Meta: 7},
	{ID: 35, Meta: 6},
	{ID: 35, Meta: 5},
	{ID: 35, Meta: 4},
	{ID: 35, Meta: 3},
	{ID: 35, Meta: 2},
	{ID: 35, Meta: 1},
	{ID: 35, Meta: 15},
	{ID: 35, Meta: 14},
	{ID: 35, Meta: 13},
	{ID: 35, Meta: 12},
	{ID: 35, Meta: 11},
	{ID: 35, Meta: 10},
	{ID: 35, Meta: 9},
	{ID: 35, Meta: 8},
	{ID: 171, Meta: 0},
	{ID: 171, Meta: 7},
	{ID: 171, Meta: 6},
	{ID: 171, Meta: 5},
	{ID: 171, Meta: 4},
	{ID: 171, Meta: 3},
	{ID: 171, Meta: 2},
	{ID: 171, Meta: 1},
	{ID: 171, Meta: 15},
	{ID: 171, Meta: 14},
	{ID: 171, Meta: 13},
	{ID: 171, Meta: 12},
	{ID: 171, Meta: 11},
	{ID: 171, Meta: 10},
	{ID: 171, Meta: 9},
	{ID: 171, Meta: 8},
	{ID: 50, Meta: 0},
	{ID: 325, Meta: 0},
	{ID: 325, Meta: 1},
	{ID: 325, Meta: 8},
	{ID: 325, Meta: 10},
	{ID: 46, Meta: 0},
	{ID: 331, Meta: 0},
	{ID: 261, Meta: 0},
	{ID: 346, Meta: 0},
	{ID: 259, Meta: 0},
	{ID: 359, Meta: 0},
	{ID: 347, Meta: 0},
	{ID: 345, Meta: 0},
	{ID: 328, Meta: 0},
	{ID: 383, Meta: 15},
	{ID: 383, Meta: 32},
	{ID: 383, Meta: 17},
	{ID: 268, Meta: 0},
	{ID: 290, Meta: 0},
	{ID: 269, Meta: 0},
	{ID: 270, Meta: 0},
	{ID: 271, Meta: 0},
	{ID: 272, Meta: 0},
	{ID: 291, Meta: 0},
	{ID: 273, Meta: 0},
	{ID: 274, Meta: 0},
	{ID: 275, Meta: 0},
	{ID: 267, Meta: 0},
	{ID: 292, Meta: 0},
	{ID: 256, Meta: 0},
	{ID: 257, Meta: 0},
	{ID: 258, Meta: 0},
	{ID: 276, Meta: 0},
	{ID: 293, Meta: 0},
	{ID: 277, Meta: 0},
	{ID: 278, Meta: 0},
	{ID: 279, Meta: 0},
	{ID: 283, Meta: 0},
	{ID: 294, Meta: 0},
	{ID: 284, Meta: 0},
	{ID: 285, Meta: 0},
	{ID: 286, Meta: 0},
	{ID: 298, Meta: 0},
	{ID: 299, Meta: 0},
	{ID: 300, Meta: 0},
	{ID: 301, Meta: 0},
	{ID: 302, Meta: 0},
	{ID: 303, Meta: 0},
	{ID: 304, Meta: 0},
	{ID: 305, Meta: 0},
	{ID: 306, Meta: 0},
	{ID: 307, Meta: 0},
	{ID: 308, Meta: 0},
	{ID: 309, Meta: 0},
	{ID: 310, Meta: 0},
	{ID: 311, Meta: 0},
	{ID: 312, Meta: 0},
	{ID: 313, Meta: 0},
	{ID: 314, Meta: 0},
	{ID: 315, Meta: 0},
	{ID: 316, Meta: 0},
	{ID: 317, Meta: 0},
	{ID: 332, Meta: 0},
	{ID: 263, Meta: 0},
	{ID: 263, Meta: 1},
	{ID: 264, Meta: 0},
	{ID: 265, Meta: 0},
	{ID: 266, Meta: 0},
	{ID: 388, Meta: 0},
	{ID: 280, Meta: 0},
	{ID: 281, Meta: 0},
	{ID: 287, Meta: 0},
	{ID: 288, Meta: 0},
	{ID: 318, Meta: 0},
	{ID: 334, Meta: 0},
	{ID: 337, Meta: 0},
	{ID: 353, Meta: 0},
	{ID: 406, Meta: 0},
	{ID: 339, Meta: 0},
	{ID: 340, Meta: 0},
	{ID: 262, Meta: 0},
	{ID: 352, Meta: 0},
	{ID: 338, Meta: 0},
	{ID: 296, Meta: 0},
	{ID: 295, Meta: 0},
	{ID: 361, Meta: 0},
	{ID: 362, Meta: 0},
	{ID: 458, Meta: 0},
	{ID: 344, Meta: 0},
	{ID: 260, Meta: 0},
	{ID: 322, Meta: 0},
	{ID: 349, Meta: 0},
	{ID: 349, Meta: 1},
	{ID: 349, Meta: 2},
	{ID: 349, Meta: 3},
	{ID: 350, Meta: 0},
	{ID: 350, Meta: 1},
	{ID: 297, Meta: 0},
	{ID: 319, Meta: 0},
	{ID: 320, Meta: 0},
	{ID: 365, Meta: 0},
	{ID: 366, Meta: 0},
	{ID: 363, Meta: 0},
	{ID: 364, Meta: 0},
	{ID: 360, Meta: 0},
	{ID: 391, Meta: 0},
	{ID: 392, Meta: 0},
	{ID: 393, Meta: 0},
	{ID: 357, Meta: 0},
	{ID: 400, Meta: 0},
	{ID: 371, Meta: 0},
	{ID: 341, Meta: 0},
	{ID: 289, Meta: 0},
	{ID: 348, Meta: 0},
	{ID: 351, Meta: 0},
	{ID: 351, Meta: 7},
	{ID: 351, Meta: 6},
	{ID: 351, Meta: 5},
	{ID: 351, Meta: 4},
	{ID: 351, Meta: 3},
	{ID: 351, Meta: 2},
	{ID: 351, Meta: 1},
	{ID: 351, Meta: 15},
	{ID: 351, Meta: 14},
	{ID: 351, Meta: 13},
	{ID: 351, Meta: 12},
	{ID: 351, Meta: 11},
	{ID: 351, Meta: 10},
	{ID: 351, Meta: 9},
	{ID: 351, Meta: 8},
}

// Item contains item data for each container slots.
type Item struct {
	ID       ID
	Meta     uint16
	Amount   byte
	Compound *nbt.Compound
}

// Read reads item data from io.Reader interface.
func (i *Item) Read(buf io.Reader) {
	i.ID = ID(ReadShort(buf))
	if i.ID == 0 {
		return
	}
	i.Amount = ReadByte(buf)
	i.Meta = ReadShort(buf)
	length := uint32(ReadLShort(buf))
	if length > 0 {
		b, _ := Read(buf, int(length))
		compound := Pool.NewBuffer(b)
		i.Compound = new(nbt.Compound)
		i.Compound.ReadFrom(compound)
	}
}

// Write returns byte slice with item data.
func (i Item) Write() []byte {
	if i.ID == 0 {
		return []byte{0, 0}
	}
	buf := Pool.NewBuffer(nil)
	WriteShort(buf, uint16(i.ID))
	WriteByte(buf, i.Amount)
	WriteShort(buf, i.Meta)
	compound := Pool.NewBuffer(nil)
	i.Compound = new(nbt.Compound)
	i.Compound.WriteTo(compound)
	WriteLShort(buf, uint16(compound.Len()))
	buf.Write(compound.Bytes())
	return buf.Bytes()
}

// Block converts the item to block struct.
// If ID is not a block ID, it panics.
func (i Item) Block() Block {
	return Block{
		ID:   i.ID.Block(),
		Meta: byte(i.Meta),
	}
}

// IsBlock returns whether the item block-convertable.
func (i Item) IsBlock() bool {
	return i.ID < 256
}
