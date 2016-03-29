package highmc

import (
	"bytes"
	"log"
	"sync/atomic"
)

// Packet IDs
const (
	LoginHead byte = 0x8f + iota
	PlayStatusHead
	DisconnectHead
	BatchHead
	TextHead
	SetTimeHead
	StartGameHead
	AddPlayerHead
	RemovePlayerHead
	AddEntityHead
	RemoveEntityHead
	AddItemEntityHead
	TakeItemEntityHead
	MoveEntityHead
	MovePlayerHead
	RemoveBlockHead
	UpdateBlockHead
	AddPaintingHead
	ExplodeHead
	LevelEventHead
	BlockEventHead
	EntityEventHead
	MobEffectHead
	UpdateAttributesHead
	MobEquipmentHead
	MobArmorEquipmentHead
	InteractHead
	UseItemHead
	PlayerActionHead
	HurtArmorHead
	SetEntityDataHead
	SetEntityMotionHead
	SetEntityLinkHead
	SetHealthHead
	SetSpawnPositionHead
	AnimateHead
	RespawnHead
	DropItemHead
	ContainerOpenHead
	ContainerCloseHead
	ContainerSetSlotHead
	ContainerSetDataHead
	ContainerSetContentHead
	CraftingDataHead
	CraftingEventHead
	AdventureSettingsHead
	BlockEntityDataHead
	_ // 0xbe is skipped: PlayerInput
	FullChunkDataHead
	SetDifficultyHead
	_ // 0xc1 is skipped: ChangeDimension
	SetPlayerGametypeHead
	PlayerListHead
	_ // TelemetryEvent
	_ // SpawnExperienceOrb
	_ // ClientboundMapItemData
	_ // MapInfoRequest
	RequestChunkRadiusHead
	ChunkRadiusUpdateHead
	_ // ItemFrameDrop
	_ // ReplaceSelectedItem
)

var packets = map[byte]MCPEPacket{
	LoginHead:               new(Login),
	PlayStatusHead:          new(PlayStatus),
	DisconnectHead:          new(Disconnect),
	BatchHead:               new(Batch),
	TextHead:                new(Text),
	SetTimeHead:             new(SetTime),
	StartGameHead:           new(StartGame),
	AddPlayerHead:           new(AddPlayer),
	RemovePlayerHead:        new(RemovePlayer),
	AddEntityHead:           new(AddEntity),
	RemoveEntityHead:        new(RemoveEntity),
	AddItemEntityHead:       new(AddItemEntity),
	TakeItemEntityHead:      new(TakeItemEntity),
	MoveEntityHead:          new(MoveEntity),
	MovePlayerHead:          new(MovePlayer),
	RemoveBlockHead:         new(RemoveBlock),
	UpdateBlockHead:         new(UpdateBlock),
	AddPaintingHead:         new(AddPainting),
	ExplodeHead:             new(Explode),
	LevelEventHead:          new(LevelEvent),
	BlockEventHead:          new(BlockEvent),
	EntityEventHead:         new(EntityEvent),
	MobEffectHead:           new(MobEffect),
	UpdateAttributesHead:    new(UpdateAttributes),
	MobEquipmentHead:        new(MobEquipment),
	MobArmorEquipmentHead:   new(MobArmorEquipment),
	InteractHead:            new(Interact),
	UseItemHead:             new(UseItem),
	PlayerActionHead:        new(PlayerAction),
	HurtArmorHead:           new(HurtArmor),
	SetEntityDataHead:       new(SetEntityData),
	SetEntityMotionHead:     new(SetEntityMotion),
	SetEntityLinkHead:       new(SetEntityLink),
	SetHealthHead:           new(SetHealth),
	SetSpawnPositionHead:    new(SetSpawnPosition),
	AnimateHead:             new(Animate),
	RespawnHead:             new(Respawn),
	DropItemHead:            new(DropItem),
	ContainerOpenHead:       new(ContainerOpen),
	ContainerCloseHead:      new(ContainerClose),
	ContainerSetSlotHead:    new(ContainerSetSlot),
	ContainerSetDataHead:    new(ContainerSetData),
	ContainerSetContentHead: new(ContainerSetContent),
	CraftingDataHead:        new(CraftingData),
	CraftingEventHead:       new(CraftingEvent),
	AdventureSettingsHead:   new(AdventureSettings),
	BlockEntityDataHead:     new(BlockEntityData),
	FullChunkDataHead:       new(FullChunkData),
	SetDifficultyHead:       new(SetDifficulty),
	SetPlayerGametypeHead:   new(SetPlayerGametype),
	PlayerListHead:          new(PlayerList),
	RequestChunkRadiusHead:  new(RequestChunkRadius),
	ChunkRadiusUpdateHead:   new(ChunkRadiusUpdate),
}

// MCPEPacket is an interface for decoding/encoding MCPE packets.
type MCPEPacket interface {
	Pid() byte
	Read(*bytes.Buffer)
	Write() *bytes.Buffer
}

// Handleable is an interface for handling received MCPE packets.
type Handleable interface {
	MCPEPacket
	Handle(*Player) error
}

// GetMCPEPacket returns MCPEPacket struct with given pid.
func GetMCPEPacket(pid byte) MCPEPacket {
	pk, _ := packets[pid]
	return pk
}

// Login needs to be documented.
type Login struct {
	Username       string
	Proto1, Proto2 uint32
	ClientID       uint64
	RawUUID        [16]byte
	ServerAddress  string
	ClientSecret   string
	SkinName       string
	Skin           []byte
}

// Pid implements MCPEPacket interface.
func (i Login) Pid() byte { return LoginHead } // 0x8f

// Read implements MCPEPacket interface.
func (i *Login) Read(buf *bytes.Buffer) {
	BatchRead(buf, &i.Username, &i.Proto1)
	if i.Proto1 < MinecraftProtocol { // Old protocol
		return
	}
	BatchRead(buf, &i.Proto2, &i.ClientID)
	copy(i.RawUUID[:], buf.Next(16))
	BatchRead(buf, &i.ServerAddress, &i.ClientSecret, &i.SkinName)
	i.Skin = []byte(ReadString(buf))
}

// Write implements MCPEPacket interface.
func (i Login) Write() *bytes.Buffer {
	buf := new(bytes.Buffer)
	BatchWrite(buf, i.Username, i.Proto1, i.Proto2,
		i.ClientID, i.RawUUID[:], i.ServerAddress,
		i.ClientSecret, i.SkinName, string(i.Skin))
	return buf
}

// Handle implements Handleable interface.
func (i Login) Handle(p *Player) (err error) {
	p.Username = i.Username
	ret := new(PlayStatus)
	if i.Proto1 > MinecraftProtocol {
		ret.Status = LoginFailedServer
		p.SendPacket(ret)
		p.Disconnect("Outdated server")
		return
	} else if i.Proto1 < MinecraftProtocol {
		ret.Status = LoginFailedClient
		p.SendPacket(ret)
		p.Disconnect("Outdated client")
		return
	}
	ret.Status = LoginSuccess
	p.SendPacket(ret)
	p.ID, p.UUID, p.Secret, p.EntityID, p.Skin, p.SkinName =
		i.ClientID, i.RawUUID, i.ClientSecret, atomic.AddUint64(&lastEntityID, 1), i.Skin, i.SkinName
	// Init pos, etc.
	if err := p.Server.RegisterPlayer(p); err != nil {
		p.Disconnect("Authentication failure", err.Error())
	}
	// Auth success!
	p.SendPacket(&StartGame{
		Seed:      0xffffffff, // -1
		Dimension: 0,
		Generator: 1, // 0: old, 1: infinite, 2: flat
		Gamemode:  1, // 0: Survival, 1: Creative
		EntityID:  0, // Player eid set to 0
		SpawnX:    0,
		SpawnY:    uint32(60),
		SpawnZ:    0,
		X:         0,
		Y:         60,
		Z:         0,
	})
	p.loggedIn = true
	p.inventory.Holder = p
	p.inventory.Init()

	go p.process()

	return
}

// Packet-specific constants
const (
	LoginSuccess uint32 = iota
	LoginFailedClient
	LoginFailedServer
	PlayerSpawn
)

// PlayStatus needs to be documented.
type PlayStatus struct {
	Status uint32
}

// Pid implements MCPEPacket interface.
func (i *PlayStatus) Pid() byte { return PlayStatusHead }

// Read implements MCPEPacket interface.
func (i *PlayStatus) Read(buf *bytes.Buffer) {
	i.Status = ReadInt(buf)
}

// Write implements MCPEPacket interface.
func (i *PlayStatus) Write() *bytes.Buffer {
	buf := new(bytes.Buffer)
	WriteInt(buf, i.Status)
	return buf
}

// Disconnect needs to be documented.
type Disconnect struct {
	Message string
}

// Pid implements MCPEPacket interface.
func (i *Disconnect) Pid() byte { return DisconnectHead }

// Read implements MCPEPacket interface.
func (i *Disconnect) Read(buf *bytes.Buffer) {
	i.Message = ReadString(buf)
}

// Write implements MCPEPacket interface.
func (i *Disconnect) Write() *bytes.Buffer {
	buf := new(bytes.Buffer)
	WriteString(buf, i.Message)
	return buf
}

// Batch needs to be documented.
type Batch struct {
	Payloads [][]byte
}

// Pid implements MCPEPacket interface.
func (i Batch) Pid() byte { return BatchHead } // 0x92

// Read implements MCPEPacket interface.
func (i *Batch) Read(buf *bytes.Buffer) {
	i.Payloads = make([][]byte, 0)
	payload, err := DecodeDeflate(buf.Next(int(ReadInt(buf))))
	if err != nil {
		log.Println("Error while decompressing Batch payload:", err)
		return
	}
	b := bytes.NewBuffer(payload)
	for b.Len() > 4 {
		size := ReadInt(b)
		pk := b.Next(int(size))
		if pk[0] == 0x92 {
			panic("Invalid BatchPacket inside BatchPacket")
		}
		i.Payloads = append(i.Payloads, pk)
	}
}

// Write implements MCPEPacket interface.
func (i Batch) Write() *bytes.Buffer {
	b := new(bytes.Buffer)
	for _, pk := range i.Payloads {
		WriteInt(b, uint32(len(pk)))
		Write(b, pk)
	}
	payload := EncodeDeflate(b.Bytes())
	buf := new(bytes.Buffer)
	BatchWrite(buf, uint32(len(payload)), payload)
	return buf
}

// Packet-specific constants
const (
	TextTypeRaw byte = iota
	TextTypeChat
	TextTypeTranslation
	TextTypePopup
	TextTypeTip
	TextTypeSystem
)

// Text needs to be documented.
type Text struct {
	TextType byte
	Source   string
	Message  string
	Params   []string
}

// Pid implements MCPEPacket interface.
func (i Text) Pid() byte { return TextHead } // 0x93

// Read implements MCPEPacket interface.
func (i *Text) Read(buf *bytes.Buffer) {
	i.TextType = ReadByte(buf)
	switch i.TextType {
	case TextTypePopup, TextTypeChat:
		ReadAny(buf, &i.Source)
		fallthrough
	case TextTypeRaw, TextTypeTip, TextTypeSystem:
		ReadAny(buf, &i.Message)
	case TextTypeTranslation:
		ReadAny(buf, &i.Message)
		cnt := ReadByte(buf)
		i.Params = make([]string, cnt)
		for k := byte(0); k < cnt; k++ {
			i.Params[k] = ReadString(buf)
		}
	}
}

// Write implements MCPEPacket interface.
func (i Text) Write() *bytes.Buffer {
	buf := new(bytes.Buffer)
	WriteByte(buf, i.TextType)
	switch i.TextType {
	case TextTypePopup, TextTypeChat:
		WriteAny(buf, i.Source)
		fallthrough
	case TextTypeRaw, TextTypeTip, TextTypeSystem:
		WriteAny(buf, i.Message)
	case TextTypeTranslation:
		WriteAny(buf, &i.Message)
		WriteByte(buf, byte(len(i.Params)))
		for _, p := range i.Params {
			WriteAny(buf, p)
		}
	}
	return buf
}

// Packet-specific constants
const (
	DayTime     = 0
	SunsetTime  = 12000
	NightTime   = 14000
	SunriseTime = 23000
	FullTime    = 24000
)

// SetTime needs to be documented.
type SetTime struct {
	Time    uint32
	Started bool
}

// Pid implements MCPEPacket interface.
func (i SetTime) Pid() byte { return SetTimeHead }

// Read implements MCPEPacket interface.
func (i *SetTime) Read(buf *bytes.Buffer) {
	i.Time = uint32((ReadInt(buf) / 19200) * FullTime)
	i.Started = ReadBool(buf)
}

// Write implements MCPEPacket interface.
func (i SetTime) Write() *bytes.Buffer {
	buf := new(bytes.Buffer)
	WriteInt(buf, uint32((i.Time*19200)/FullTime))
	WriteBool(buf, i.Started)
	return buf
}

// StartGame needs to be documented.
type StartGame struct {
	Seed                   uint32
	Dimension              byte
	Generator              uint32
	Gamemode               uint32
	EntityID               uint64
	SpawnX, SpawnY, SpawnZ uint32
	X, Y, Z                float32
}

// Pid implements MCPEPacket interface.
func (i StartGame) Pid() byte { return StartGameHead } // 0x95

// Read implements MCPEPacket interface.
func (i *StartGame) Read(buf *bytes.Buffer) {
	BatchRead(buf, &i.Seed, &i.Dimension, &i.Generator,
		&i.Gamemode, &i.EntityID, &i.SpawnX,
		&i.SpawnY, &i.SpawnZ, &i.X,
		&i.Y, &i.Z)
}

// Write implements MCPEPacket interface.
func (i StartGame) Write() *bytes.Buffer {
	buf := new(bytes.Buffer)
	BatchWrite(buf, i.Seed, i.Dimension, i.Generator,
		i.Gamemode, i.EntityID, i.SpawnX,
		i.SpawnY, i.SpawnZ, i.X,
		i.Y, i.Z)
	WriteByte(buf, 0)
	return buf
}

// AddPlayer needs to be documented.
type AddPlayer struct {
	RawUUID                [16]byte
	Username               string
	EntityID               uint64
	X, Y, Z                float32
	SpeedX, SpeedY, SpeedZ float32
	BodyYaw, Yaw, Pitch    float32
	Metadata               []byte
}

// Pid implements MCPEPacket interface.
func (i AddPlayer) Pid() byte { return AddPlayerHead }

// Read implements MCPEPacket interface.
func (i *AddPlayer) Read(buf *bytes.Buffer) {
	copy(i.RawUUID[:], buf.Next(16))
	BatchRead(buf, &i.Username, &i.EntityID,
		&i.X, &i.Y, &i.Z,
		&i.SpeedX, &i.SpeedY, &i.SpeedZ,
		&i.BodyYaw, &i.Yaw, &i.Pitch)
	i.Metadata = buf.Bytes()
}

// Write implements MCPEPacket interface.
func (i AddPlayer) Write() *bytes.Buffer {
	buf := new(bytes.Buffer)
	BatchWrite(buf, i.RawUUID[:], i.Username, i.EntityID,
		i.X, i.Y, i.Z,
		i.SpeedX, i.SpeedY, i.SpeedZ,
		i.BodyYaw, i.Yaw, i.Pitch, i.Metadata)
	WriteByte(buf, 0x7f) // Temporal, TODO: implement metadata functions
	return buf
}

// RemovePlayer needs to be documented.
type RemovePlayer struct {
	EntityID uint64
	RawUUID  [16]byte
}

// Pid implements MCPEPacket interface.
func (i RemovePlayer) Pid() byte { return RemovePlayerHead }

// Read implements MCPEPacket interface.
func (i *RemovePlayer) Read(buf *bytes.Buffer) {
	i.EntityID = ReadLong(buf)
	copy(i.RawUUID[:], buf.Next(16))
}

// Write implements MCPEPacket interface.
func (i RemovePlayer) Write() *bytes.Buffer {
	buf := new(bytes.Buffer)
	WriteLong(buf, i.EntityID)
	buf.Write(i.RawUUID[:])
	return buf
}

// AddEntity needs to be documented.
type AddEntity struct {
	EntityID               uint64
	Type                   uint32
	X, Y, Z                float32
	SpeedX, SpeedY, SpeedZ float32
	Yaw, Pitch             float32
	Metadata               []byte
	Link1, Link2           uint64
	Link3                  byte
}

// Pid implements MCPEPacket interface.
func (i AddEntity) Pid() byte { return AddEntityHead }

// Read implements MCPEPacket interface.
func (i *AddEntity) Read(buf *bytes.Buffer) {
	BatchRead(buf, &i.EntityID, &i.Type,
		&i.X, &i.Y, &i.Z,
		&i.SpeedX, &i.SpeedY, &i.SpeedZ,
		&i.Yaw, &i.Pitch)
	i.Metadata = buf.Bytes()
	// TODO
}

// Write implements MCPEPacket interface.
func (i AddEntity) Write() *bytes.Buffer {
	buf := new(bytes.Buffer)
	BatchWrite(buf, i.EntityID, i.Type,
		i.X, i.Y, i.Z,
		i.SpeedX, i.SpeedY, i.SpeedZ,
		i.Yaw, i.Pitch)
	WriteByte(buf, 0x7f)
	BatchWrite(buf, i.Link1, i.Link2, i.Link3)
	return buf
}

// RemoveEntity needs to be documented.
type RemoveEntity struct {
	EntityID uint64
}

// Pid implements MCPEPacket interface.
func (i RemoveEntity) Pid() byte { return RemoveEntityHead }

// Read implements MCPEPacket interface.
func (i *RemoveEntity) Read(buf *bytes.Buffer) {
	i.EntityID = ReadLong(buf)
}

// Write implements MCPEPacket interface.
func (i RemoveEntity) Write() *bytes.Buffer {
	buf := new(bytes.Buffer)
	WriteLong(buf, i.EntityID)
	return buf
}

// AddItemEntity needs to be documented.
type AddItemEntity struct {
	EntityID uint64
	Item     *Item
	X        float32
	Y        float32
	Z        float32
	SpeedX   float32
	SpeedY   float32
	SpeedZ   float32
}

// Pid implements MCPEPacket interface.
func (i AddItemEntity) Pid() byte { return AddItemEntityHead }

// Read implements MCPEPacket interface.
func (i *AddItemEntity) Read(buf *bytes.Buffer) {
	i.EntityID = ReadLong(buf)
	i.Item = new(Item)
	i.Item.Read(buf)
	i.X = ReadFloat(buf)
	i.Y = ReadFloat(buf)
	i.Z = ReadFloat(buf)
	i.SpeedX = ReadFloat(buf)
	i.SpeedY = ReadFloat(buf)
	i.SpeedZ = ReadFloat(buf)
}

// Write implements MCPEPacket interface.
func (i AddItemEntity) Write() *bytes.Buffer {
	buf := new(bytes.Buffer)
	WriteLong(buf, i.EntityID)
	buf.Write(i.Item.Write())
	WriteFloat(buf, i.X)
	WriteFloat(buf, i.Y)
	WriteFloat(buf, i.Z)
	WriteFloat(buf, i.SpeedX)
	WriteFloat(buf, i.SpeedY)
	WriteFloat(buf, i.SpeedZ)
	return buf
}

// TakeItemEntity needs to be documented.
type TakeItemEntity struct {
	Target   uint64
	EntityID uint64
}

// Pid implements MCPEPacket interface.
func (i TakeItemEntity) Pid() byte { return TakeItemEntityHead }

// Read implements MCPEPacket interface.
func (i *TakeItemEntity) Read(buf *bytes.Buffer) {
	i.Target = ReadLong(buf)
	i.EntityID = ReadLong(buf)
}

// Write implements MCPEPacket interface.
func (i TakeItemEntity) Write() *bytes.Buffer {
	buf := new(bytes.Buffer)
	WriteLong(buf, i.Target)
	WriteLong(buf, i.EntityID)
	return buf
}

// MoveEntity needs to be documented.
type MoveEntity struct {
	EntityIDs []uint64
	EntityPos [][6]float32 // X, Y, Z, Yaw, HeadYaw, Pitch
}

// Pid implements MCPEPacket interface.
func (i MoveEntity) Pid() byte { return MoveEntityHead }

// Read implements MCPEPacket interface.
func (i *MoveEntity) Read(buf *bytes.Buffer) {
	entityCnt := ReadInt(buf)
	i.EntityIDs = make([]uint64, entityCnt)
	i.EntityPos = make([][6]float32, entityCnt)
	for j := uint32(0); j < entityCnt; j++ {
		i.EntityIDs[j] = ReadLong(buf)
		for k := 0; k < 6; k++ {
			i.EntityPos[j][k] = ReadFloat(buf)
		}
	}
}

// Write implements MCPEPacket interface.
func (i MoveEntity) Write() *bytes.Buffer {
	if len(i.EntityIDs) != len(i.EntityPos) {
		panic("Entity data slice length mismatch")
	}
	buf := new(bytes.Buffer)
	WriteInt(buf, uint32(len(i.EntityIDs)))
	for k, e := range i.EntityIDs {
		WriteLong(buf, e)
		for j := 0; j < 6; j++ {
			WriteFloat(buf, i.EntityPos[k][j])
		}
	}
	return buf
}

// Packet-specific constants
const (
	ModeNormal   byte = 0
	ModeReset    byte = 1
	ModeRotation byte = 2
)

// MovePlayer needs to be documented.
type MovePlayer struct {
	EntityID uint64
	X        float32
	Y        float32
	Z        float32
	Yaw      float32
	BodyYaw  float32
	Pitch    float32
	Mode     byte
	OnGround byte
}

// Pid implements MCPEPacket interface.
func (i MovePlayer) Pid() byte { return MovePlayerHead }

// Read implements MCPEPacket interface.
func (i *MovePlayer) Read(buf *bytes.Buffer) {
	i.EntityID = ReadLong(buf)
	i.X = ReadFloat(buf)
	i.Y = ReadFloat(buf)
	i.Z = ReadFloat(buf)
	i.Yaw = ReadFloat(buf)
	i.BodyYaw = ReadFloat(buf)
	i.Pitch = ReadFloat(buf)
	i.Mode = ReadByte(buf)
	i.OnGround = ReadByte(buf)
}

// Write implements MCPEPacket interface.
func (i MovePlayer) Write() *bytes.Buffer {
	buf := new(bytes.Buffer)
	WriteLong(buf, i.EntityID)
	WriteFloat(buf, i.X)
	WriteFloat(buf, i.Y)
	WriteFloat(buf, i.Z)
	WriteFloat(buf, i.Yaw)
	WriteFloat(buf, i.BodyYaw)
	WriteFloat(buf, i.Pitch)
	WriteByte(buf, i.Mode)
	WriteByte(buf, i.OnGround)
	return buf
}

// RemoveBlock needs to be documented.
type RemoveBlock struct {
	EntityID uint64
	X, Z     uint32
	Y        byte
}

// Pid implements MCPEPacket interface.
func (i RemoveBlock) Pid() byte { return RemoveBlockHead }

// Read implements MCPEPacket interface.
func (i *RemoveBlock) Read(buf *bytes.Buffer) {
	i.EntityID = ReadLong(buf)
	i.X = ReadInt(buf)
	i.Z = ReadInt(buf)
	i.Y = ReadByte(buf)
}

// Write implements MCPEPacket interface.
func (i RemoveBlock) Write() *bytes.Buffer {
	buf := new(bytes.Buffer)
	WriteLong(buf, i.EntityID)
	WriteInt(buf, i.X)
	WriteInt(buf, i.Z)
	WriteByte(buf, i.Y)
	return buf
}

// Packet-specific constants
const (
	UpdateNone byte = (1 << iota) >> 1
	UpdateNeighbors
	UpdateNetwork
	UpdateNographic
	UpdatePriority
	UpdateAll         = UpdateNeighbors | UpdateNetwork
	UpdateAllPriority = UpdateAll | UpdatePriority
)

// BlockRecord needs to be documented.
type BlockRecord struct {
	X, Z  uint32
	Y     byte
	Block Block
	Flags byte
}

// UpdateBlock needs to be documented.
type UpdateBlock struct {
	BlockRecords []BlockRecord
}

// Pid implements MCPEPacket interface.
func (i UpdateBlock) Pid() byte { return UpdateBlockHead }

// Read implements MCPEPacket interface.
func (i *UpdateBlock) Read(buf *bytes.Buffer) {
	records := ReadInt(buf)
	i.BlockRecords = make([]BlockRecord, records)
	for k := uint32(0); k < records; k++ {
		x := ReadInt(buf)
		z := ReadInt(buf)
		y := ReadByte(buf)
		id := ReadByte(buf)
		flagMeta := ReadByte(buf)
		i.BlockRecords[k] = BlockRecord{X: x,
			Y: y,
			Z: z,
			Block: Block{
				ID:   id,
				Meta: flagMeta & 0x0f,
			},
			Flags: (flagMeta >> 4) & 0x0f,
		}
	}
}

// Write implements MCPEPacket interface.
func (i UpdateBlock) Write() *bytes.Buffer {
	buf := new(bytes.Buffer)
	WriteInt(buf, uint32(len(i.BlockRecords)))
	for _, record := range i.BlockRecords {
		BatchWrite(buf, record.X, record.Z, record.Y, record.Block.ID, (record.Flags<<4 | record.Block.Meta))
	}
	return buf
}

// AddPainting needs to be documented.
type AddPainting struct {
	EntityID  uint64
	X         uint32
	Y         uint32
	Z         uint32
	Direction uint32
	Title     string
}

// Pid implements MCPEPacket interface.
func (i AddPainting) Pid() byte { return AddPaintingHead }

// Read implements MCPEPacket interface.
func (i *AddPainting) Read(buf *bytes.Buffer) {
	i.EntityID = ReadLong(buf)
	i.X = ReadInt(buf)
	i.Y = ReadInt(buf)
	i.Z = ReadInt(buf)
	i.Direction = ReadInt(buf)
	i.Title = ReadString(buf)
}

// Write implements MCPEPacket interface.
func (i AddPainting) Write() *bytes.Buffer {
	buf := new(bytes.Buffer)
	WriteLong(buf, i.EntityID)
	WriteInt(buf, i.X)
	WriteInt(buf, i.Y)
	WriteInt(buf, i.Z)
	WriteInt(buf, i.Direction)
	WriteString(buf, i.Title)
	return buf
}

// Explode needs to be documented.
type Explode struct {
	X, Y, Z, Radius float32
	Records         [][3]byte // X, Y, Z byte
}

// Pid implements MCPEPacket interface.
func (i Explode) Pid() byte { return ExplodeHead }

// Read implements MCPEPacket interface.
func (i *Explode) Read(buf *bytes.Buffer) {
	BatchRead(buf, &i.X, &i.Y, &i.Z, &i.Radius)
	cnt := ReadInt(buf)
	i.Records = make([][3]byte, cnt)
	for k := uint32(0); k < cnt; k++ {
		BatchRead(buf, &i.Records[k][0], &i.Records[k][1], &i.Records[k][2])
	}
}

// Write implements MCPEPacket interface.
func (i Explode) Write() *bytes.Buffer {
	buf := new(bytes.Buffer)
	BatchWrite(buf, i.X, i.Y, i.Z, i.Radius)
	WriteInt(buf, uint32(len(i.Records)))
	for _, r := range i.Records {
		BatchWrite(buf, r[0], r[1], r[2])
	}
	return buf
}

// Packet-specific constants
const (
	EventSoundClick            = 1000
	EventSoundClickFail        = 1001
	EventSoundShoot            = 1002
	EventSoundDoor             = 1003
	EventSoundFizz             = 1004
	EventSoundGhast            = 1007
	EventSoundGhastShoot       = 1008
	EventSoundBlazeShoot       = 1009
	EventSoundDoorBump         = 1010
	EventSoundDoorCrash        = 1012
	EventSoundBatFly           = 1015
	EventSoundZombieInfect     = 1016
	EventSoundZombieHeal       = 1017
	EventSoundEndermanTeleport = 1018
	EventSoundAnvilBreak       = 1020
	EventSoundAnvilUse         = 1021
	EventSoundAnvilFall        = 1022
	EventParticleShoot         = 2000
	EventParticleDestroy       = 2001
	EventParticleSplash        = 2002
	EventParticleEyeDespawn    = 2003
	EventParticleSpawn         = 2004
	EventStartRain             = 3001
	EventStartThunder          = 3002
	EventStopRain              = 3003
	EventStopThunder           = 3004
	EventSetData               = 4000
	EventPlayersSleeping       = 9800
)

// LevelEvent needs to be documented.
type LevelEvent struct {
	EventID uint16
	X       float32
	Y       float32
	Z       float32
	Data    uint32
}

// Pid implements MCPEPacket interface.
func (i LevelEvent) Pid() byte { return LevelEventHead }

// Read implements MCPEPacket interface.
func (i *LevelEvent) Read(buf *bytes.Buffer) {
	i.EventID = ReadShort(buf)
	i.X = ReadFloat(buf)
	i.Y = ReadFloat(buf)
	i.Z = ReadFloat(buf)
	i.Data = ReadInt(buf)
}

// Write implements MCPEPacket interface.
func (i LevelEvent) Write() *bytes.Buffer {
	buf := new(bytes.Buffer)
	WriteShort(buf, i.EventID)
	WriteFloat(buf, i.X)
	WriteFloat(buf, i.Y)
	WriteFloat(buf, i.Z)
	WriteInt(buf, i.Data)
	return buf
}

// BlockEvent needs to be documented.
type BlockEvent struct {
	X     uint32
	Y     uint32
	Z     uint32
	Case1 uint32
	Case2 uint32
}

// Pid implements MCPEPacket interface.
func (i BlockEvent) Pid() byte { return BlockEventHead }

// Read implements MCPEPacket interface.
func (i *BlockEvent) Read(buf *bytes.Buffer) {
	i.X = ReadInt(buf)
	i.Y = ReadInt(buf)
	i.Z = ReadInt(buf)
	i.Case1 = ReadInt(buf)
	i.Case2 = ReadInt(buf)
}

// Write implements MCPEPacket interface.
func (i BlockEvent) Write() *bytes.Buffer {
	buf := new(bytes.Buffer)
	WriteInt(buf, i.X)
	WriteInt(buf, i.Y)
	WriteInt(buf, i.Z)
	WriteInt(buf, i.Case1)
	WriteInt(buf, i.Case2)
	return buf
}

// Packet-specific constants
const (
	EventHurtAnimation byte = iota + 2
	EventDeathAnimation
	_
	_
	EventTameFail
	EventTameSuccess
	EventShakeWet
	EventUseItem
	EventEatGrassAnimation
	EventFishHookBubble
	EventFishHookPosition
	EventFishHookHook
	EventFishHookTease
	EventSquidInkCloud
	EventAmbientSound
	EventRespawn
)

// EntityEvent needs to be documented.
type EntityEvent struct {
	EntityID uint64
	Event    byte
}

// Pid implements MCPEPacket interface.
func (i EntityEvent) Pid() byte { return EntityEventHead }

// Read implements MCPEPacket interface.
func (i *EntityEvent) Read(buf *bytes.Buffer) {
	i.EntityID = ReadLong(buf)
	i.Event = ReadByte(buf)
}

// Write implements MCPEPacket interface.
func (i EntityEvent) Write() *bytes.Buffer {
	buf := new(bytes.Buffer)
	WriteLong(buf, i.EntityID)
	WriteByte(buf, i.Event)
	return buf
}

// Packet-specific constants
const (
	EffectAdd byte = iota + 1
	EffectModify
	EffectRemove
)

// MobEffect needs to be documented.
type MobEffect struct {
	EntityID  uint64
	EventID   byte
	EffectID  byte
	Amplifier byte
	Particles bool
	Duration  uint32
}

// Pid implements MCPEPacket interface.
func (i MobEffect) Pid() byte { return MobEffectHead }

// Read implements MCPEPacket interface.
func (i *MobEffect) Read(buf *bytes.Buffer) {
	i.EntityID = ReadLong(buf)
	i.EventID = ReadByte(buf)
	i.EffectID = ReadByte(buf)
	i.Amplifier = ReadByte(buf)
	i.Particles = ReadBool(buf)
	i.Duration = ReadInt(buf)
}

// Write implements MCPEPacket interface.
func (i MobEffect) Write() *bytes.Buffer {
	buf := new(bytes.Buffer)
	WriteLong(buf, i.EntityID)
	WriteByte(buf, i.EventID)
	WriteByte(buf, i.EffectID)
	WriteByte(buf, i.Amplifier)
	WriteBool(buf, i.Particles)
	WriteInt(buf, i.Duration)
	return buf
}

// UpdateAttributes needs to be documented.
type UpdateAttributes struct {
	// TODO: implement this after NBT is done
}

// Pid implements MCPEPacket interface.
func (i UpdateAttributes) Pid() byte { return UpdateAttributesHead }

// Read implements MCPEPacket interface.
func (i *UpdateAttributes) Read(buf *bytes.Buffer) {}

// Write implements MCPEPacket interface.
func (i UpdateAttributes) Write() *bytes.Buffer { return nil }

// MobEquipment needs to be documented.
type MobEquipment struct {
	EntityID     uint64
	Item         *Item
	Slot         byte
	SelectedSlot byte
}

// Pid implements MCPEPacket interface.
func (i MobEquipment) Pid() byte { return MobEquipmentHead }

// Read implements MCPEPacket interface.
func (i *MobEquipment) Read(buf *bytes.Buffer) {
	i.EntityID = ReadLong(buf)
	i.Item = new(Item)
	i.Item.Read(buf)
	i.Slot = ReadByte(buf)
	i.SelectedSlot = ReadByte(buf)
}

// Write implements MCPEPacket interface.
func (i MobEquipment) Write() *bytes.Buffer {
	buf := new(bytes.Buffer)
	WriteLong(buf, i.EntityID)
	buf.Write(i.Item.Write())
	WriteByte(buf, i.Slot)
	WriteByte(buf, i.SelectedSlot)
	return buf
}

// MobArmorEquipment needs to be documented.
type MobArmorEquipment struct {
	EntityID uint64
	Slots    [4]*Item
}

// Pid implements MCPEPacket interface.
func (i MobArmorEquipment) Pid() byte { return MobArmorEquipmentHead }

// Read implements MCPEPacket interface.
func (i *MobArmorEquipment) Read(buf *bytes.Buffer) {
	i.EntityID = ReadLong(buf)
	for j := range i.Slots {
		i.Slots[j] = new(Item)
		i.Slots[j].Read(buf)
	}
}

// Write implements MCPEPacket interface.
func (i MobArmorEquipment) Write() *bytes.Buffer {
	buf := new(bytes.Buffer)
	WriteLong(buf, i.EntityID)
	for j := range i.Slots {
		buf.Write(i.Slots[j].Write())
	}
	return buf
}

// Interact needs to be documented.
type Interact struct {
	Action byte
	Target uint64
}

// Pid implements MCPEPacket interface.
func (i Interact) Pid() byte { return InteractHead }

// Read implements MCPEPacket interface.
func (i *Interact) Read(buf *bytes.Buffer) {
	i.Action = ReadByte(buf)
	i.Target = ReadLong(buf)
}

// Write implements MCPEPacket interface.
func (i Interact) Write() *bytes.Buffer {
	buf := new(bytes.Buffer)
	WriteByte(buf, i.Action)
	WriteLong(buf, i.Target)
	return buf
}

// UseItem needs to be documented.
type UseItem struct {
	X, Y, Z                uint32
	Face                   byte
	FloatX, FloatY, FloatZ float32
	PosX, PosY, PosZ       float32
	Item                   *Item
}

// Pid implements MCPEPacket interface.
func (i UseItem) Pid() byte { return UseItemHead }

// Read implements MCPEPacket interface.
func (i *UseItem) Read(buf *bytes.Buffer) {
	BatchRead(buf, &i.X, &i.Y, &i.Z,
		&i.Face, &i.FloatX, &i.FloatY, &i.FloatZ,
		&i.PosX, &i.PosY, &i.PosZ)
	i.Item = new(Item)
	i.Item.Read(buf)
}

// Write implements MCPEPacket interface.
func (i UseItem) Write() *bytes.Buffer {
	buf := new(bytes.Buffer)
	BatchWrite(buf, i.X, i.Y, i.Z,
		i.Face, i.FloatX, i.FloatY, i.FloatZ,
		i.PosX, i.PosY, i.PosZ, i.Item.Write())
	return buf
}

// Packet-specific constants
const (
	ActionStartBreak uint32 = iota
	ActionAbortBreak
	ActionStopBreak
	_
	_
	ActionReleaseItem
	ActionStopSleeping
	ActionRespawn
	ActionJump
	ActionStartSprint
	ActionStopSprint
	ActionStartSneak
	ActionStopSneak
	ActionDimensionChange
)

// PlayerAction needs to be documented.
type PlayerAction struct {
	EntityID uint64
	Action   uint32
	X        uint32
	Y        uint32
	Z        uint32
	Face     uint32
}

// Pid implements MCPEPacket interface.
func (i PlayerAction) Pid() byte { return PlayerActionHead }

// Read implements MCPEPacket interface.
func (i *PlayerAction) Read(buf *bytes.Buffer) {
	i.EntityID = ReadLong(buf)
	i.Action = ReadInt(buf)
	i.X = ReadInt(buf)
	i.Y = ReadInt(buf)
	i.Z = ReadInt(buf)
	i.Face = ReadInt(buf)
}

// Write implements MCPEPacket interface.
func (i PlayerAction) Write() *bytes.Buffer {
	buf := new(bytes.Buffer)
	WriteLong(buf, i.EntityID)
	WriteInt(buf, i.Action)
	WriteInt(buf, i.X)
	WriteInt(buf, i.Y)
	WriteInt(buf, i.Z)
	WriteInt(buf, i.Face)
	return buf
}

// HurtArmor needs to be documented.
type HurtArmor struct {
	Health byte
}

// Pid implements MCPEPacket interface.
func (i HurtArmor) Pid() byte { return HurtArmorHead }

// Read implements MCPEPacket interface.
func (i *HurtArmor) Read(buf *bytes.Buffer) {
	i.Health = ReadByte(buf)
}

// Write implements MCPEPacket interface.
func (i HurtArmor) Write() *bytes.Buffer {
	buf := new(bytes.Buffer)
	WriteByte(buf, i.Health)
	return buf
}

// SetEntityData needs to be documented.
type SetEntityData struct{} // TODO Metadata

// Pid implements MCPEPacket interface.
func (i SetEntityData) Pid() byte { return SetEntityDataHead }

// Read implements MCPEPacket interface.
func (i *SetEntityData) Read(buf *bytes.Buffer) {}

// Write implements MCPEPacket interface.
func (i SetEntityData) Write() *bytes.Buffer {
	return nil
}

// SetEntityMotion needs to be documented.
type SetEntityMotion struct {
	EntityIDs    []uint64
	EntityMotion [][6]float32 // X, Y, Z, Yaw, HeadYaw, Pitch
}

// Pid implements MCPEPacket interface.
func (i SetEntityMotion) Pid() byte { return SetEntityMotionHead }

// Read implements MCPEPacket interface.
func (i *SetEntityMotion) Read(buf *bytes.Buffer) {
	entityCnt := ReadInt(buf)
	i.EntityIDs = make([]uint64, entityCnt)
	i.EntityMotion = make([][6]float32, entityCnt)
	for j := uint32(0); j < entityCnt; j++ {
		i.EntityIDs[j] = ReadLong(buf)
		for k := 0; k < 6; k++ {
			i.EntityMotion[j][k] = ReadFloat(buf)
		}
	}
}

// Write implements MCPEPacket interface.
func (i SetEntityMotion) Write() *bytes.Buffer {
	if len(i.EntityIDs) != len(i.EntityMotion) {
		panic("Entity data slice length mismatch")
	}
	buf := new(bytes.Buffer)
	WriteInt(buf, uint32(len(i.EntityIDs)))
	for k, e := range i.EntityIDs {
		WriteLong(buf, e)
		for j := 0; j < 6; j++ {
			WriteFloat(buf, i.EntityMotion[k][j])
		}
	}
	return buf
}

// SetEntityLink needs to be documented.
type SetEntityLink struct {
	From uint64
	To   uint64
	Type byte
}

// Pid implements MCPEPacket interface.
func (i SetEntityLink) Pid() byte { return SetEntityLinkHead }

// Read implements MCPEPacket interface.
func (i *SetEntityLink) Read(buf *bytes.Buffer) {
	i.From = ReadLong(buf)
	i.To = ReadLong(buf)
	i.Type = ReadByte(buf)
}

// Write implements MCPEPacket interface.
func (i SetEntityLink) Write() *bytes.Buffer {
	buf := new(bytes.Buffer)
	WriteLong(buf, i.From)
	WriteLong(buf, i.To)
	WriteByte(buf, i.Type)
	return buf
}

// SetHealth needs to be documented.
type SetHealth struct {
	Health uint32
}

// Pid implements MCPEPacket interface.
func (i SetHealth) Pid() byte { return SetHealthHead }

// Read implements MCPEPacket interface.
func (i *SetHealth) Read(buf *bytes.Buffer) {
	i.Health = ReadInt(buf)
}

// Write implements MCPEPacket interface.
func (i SetHealth) Write() *bytes.Buffer {
	buf := new(bytes.Buffer)
	WriteInt(buf, i.Health)
	return buf
}

// SetSpawnPosition needs to be documented.
type SetSpawnPosition struct {
	X uint32
	Y uint32
	Z uint32
}

// Pid implements MCPEPacket interface.
func (i SetSpawnPosition) Pid() byte { return SetSpawnPositionHead }

// Read implements MCPEPacket interface.
func (i *SetSpawnPosition) Read(buf *bytes.Buffer) {
	i.X = ReadInt(buf)
	i.Y = ReadInt(buf)
	i.Z = ReadInt(buf)
}

// Write implements MCPEPacket interface.
func (i SetSpawnPosition) Write() *bytes.Buffer {
	buf := new(bytes.Buffer)
	WriteInt(buf, i.X)
	WriteInt(buf, i.Y)
	WriteInt(buf, i.Z)
	return buf
}

// Animate needs to be documented.
type Animate struct {
	Action   byte
	EntityID uint64
}

// Pid implements MCPEPacket interface.
func (i Animate) Pid() byte { return AnimateHead }

// Read implements MCPEPacket interface.
func (i *Animate) Read(buf *bytes.Buffer) {
	i.Action = ReadByte(buf)
	i.EntityID = ReadLong(buf)
}

// Write implements MCPEPacket interface.
func (i Animate) Write() *bytes.Buffer {
	buf := new(bytes.Buffer)
	WriteByte(buf, i.Action)
	WriteLong(buf, i.EntityID)
	return buf
}

// Respawn needs to be documented.
type Respawn struct {
	X float32
	Y float32
	Z float32
}

// Pid implements MCPEPacket interface.
func (i Respawn) Pid() byte { return RespawnHead }

// Read implements MCPEPacket interface.
func (i *Respawn) Read(buf *bytes.Buffer) {
	i.X = ReadFloat(buf)
	i.Y = ReadFloat(buf)
	i.Z = ReadFloat(buf)
}

// Write implements MCPEPacket interface.
func (i Respawn) Write() *bytes.Buffer {
	buf := new(bytes.Buffer)
	WriteFloat(buf, i.X)
	WriteFloat(buf, i.Y)
	WriteFloat(buf, i.Z)
	return buf
}

// DropItem needs to be documented.
type DropItem struct {
	Type byte
	Item *Item
}

// Pid implements MCPEPacket interface.
func (i DropItem) Pid() byte { return DropItemHead }

// Read implements MCPEPacket interface.
func (i *DropItem) Read(buf *bytes.Buffer) {
	i.Type = ReadByte(buf)
	i.Item = new(Item)
	i.Item.Read(buf)
}

// Write implements MCPEPacket interface.
func (i DropItem) Write() *bytes.Buffer {
	buf := new(bytes.Buffer)
	BatchWrite(buf, i.Type, i.Item.Write())
	return buf
}

// ContainerOpen needs to be documented.
type ContainerOpen struct {
	WindowID byte
	Type     byte
	Slots    uint16
	X        uint32
	Y        uint32
	Z        uint32
}

// Pid implements MCPEPacket interface.
func (i ContainerOpen) Pid() byte { return ContainerOpenHead }

// Read implements MCPEPacket interface.
func (i *ContainerOpen) Read(buf *bytes.Buffer) {
	i.WindowID = ReadByte(buf)
	i.Type = ReadByte(buf)
	i.Slots = ReadShort(buf)
	i.X = ReadInt(buf)
	i.Y = ReadInt(buf)
	i.Z = ReadInt(buf)
}

// Write implements MCPEPacket interface.
func (i ContainerOpen) Write() *bytes.Buffer {
	buf := new(bytes.Buffer)
	WriteByte(buf, i.WindowID)
	WriteByte(buf, i.Type)
	WriteShort(buf, i.Slots)
	WriteInt(buf, i.X)
	WriteInt(buf, i.Y)
	WriteInt(buf, i.Z)
	return buf
}

// ContainerClose needs to be documented.
type ContainerClose struct {
	WindowID byte
}

// Pid implements MCPEPacket interface.
func (i ContainerClose) Pid() byte { return ContainerCloseHead }

// Read implements MCPEPacket interface.
func (i *ContainerClose) Read(buf *bytes.Buffer) {
	i.WindowID = ReadByte(buf)
}

// Write implements MCPEPacket interface.
func (i ContainerClose) Write() *bytes.Buffer {
	buf := new(bytes.Buffer)
	WriteByte(buf, i.WindowID)
	return buf
}

// ContainerSetSlot needs to be documented.
type ContainerSetSlot struct { // TODO: implement this after slots
	Windowid   byte
	Slot       uint16
	HotbarSlot uint16
	Item       *Item
}

// Pid implements MCPEPacket interface.
func (i ContainerSetSlot) Pid() byte { return ContainerSetSlotHead }

// Read implements MCPEPacket interface.
func (i *ContainerSetSlot) Read(buf *bytes.Buffer) {
	i.Windowid = ReadByte(buf)
	i.Slot = ReadShort(buf)
	i.HotbarSlot = ReadShort(buf)
	i.Item = new(Item)
	i.Item.Read(buf)
}

// Write implements MCPEPacket interface.
func (i ContainerSetSlot) Write() *bytes.Buffer {
	buf := new(bytes.Buffer)
	WriteByte(buf, i.Windowid)
	WriteShort(buf, i.Slot)
	WriteShort(buf, i.HotbarSlot)
	buf.Write(i.Item.Write())
	return buf
}

// ContainerSetData needs to be documented.
type ContainerSetData struct {
	WindowID byte
	Property uint16
	Value    uint16
}

// Pid implements MCPEPacket interface.
func (i ContainerSetData) Pid() byte { return ContainerSetDataHead }

// Read implements MCPEPacket interface.
func (i *ContainerSetData) Read(buf *bytes.Buffer) {
	i.WindowID = ReadByte(buf)
	i.Property = ReadShort(buf)
	i.Value = ReadShort(buf)
}

// Write implements MCPEPacket interface.
func (i ContainerSetData) Write() *bytes.Buffer {
	buf := new(bytes.Buffer)
	WriteByte(buf, i.WindowID)
	WriteShort(buf, i.Property)
	WriteShort(buf, i.Value)
	return buf
}

// Packet-specific constants
const (
	InventoryWindow byte = 0
	ArmorWindow     byte = 0x78
	CreativeWindow  byte = 0x79
)

// ContainerSetContent needs to be documented.
type ContainerSetContent struct {
	WindowID byte
	Slots    []Item
	Hotbar   []uint32
}

// Pid implements MCPEPacket interface.
func (i ContainerSetContent) Pid() byte { return ContainerSetContentHead }

// Read implements MCPEPacket interface.
func (i *ContainerSetContent) Read(buf *bytes.Buffer) {
	i.WindowID = ReadByte(buf)
	count := ReadShort(buf)
	i.Slots = make([]Item, count)
	for j := range i.Slots {
		if buf.Len() < 7 {
			break
		}
		i.Slots[j] = *new(Item)
		(&i.Slots[j]).Read(buf)
	}
	if i.WindowID == InventoryWindow {
		count := ReadShort(buf)
		i.Hotbar = make([]uint32, count)
		for j := range i.Hotbar {
			i.Hotbar[j] = ReadInt(buf)
		}
	}
}

// Write implements MCPEPacket interface.
func (i ContainerSetContent) Write() *bytes.Buffer {
	buf := new(bytes.Buffer)
	WriteByte(buf, i.WindowID)
	WriteShort(buf, uint16(len(i.Slots)))
	for _, slot := range i.Slots {
		Write(buf, slot.Write())
	}
	if i.WindowID == InventoryWindow {
		for _, h := range i.Hotbar {
			WriteInt(buf, h)
		}
	} else {
		WriteShort(buf, 0)
	}
	return buf
}

// CraftingData needs to be documented.
type CraftingData struct{} // TODO

// Pid implements MCPEPacket interface.
func (i CraftingData) Pid() byte { return CraftingDataHead }

// Read implements MCPEPacket interface.
func (i *CraftingData) Read(buf *bytes.Buffer) {}

// Write implements MCPEPacket interface.
func (i CraftingData) Write() *bytes.Buffer { return nil }

// CraftingEvent needs to be documented.
type CraftingEvent struct{} // TODO

// Pid implements MCPEPacket interface.
func (i CraftingEvent) Pid() byte { return CraftingEventHead }

// Read implements MCPEPacket interface.
func (i *CraftingEvent) Read(buf *bytes.Buffer) {}

// Write implements MCPEPacket interface.
func (i CraftingEvent) Write() *bytes.Buffer { return nil }

// AdventureSettings needs to be documented.
type AdventureSettings struct {
	Flags uint32
}

// Pid implements MCPEPacket interface.
func (i AdventureSettings) Pid() byte { return AdventureSettingsHead }

// Read implements MCPEPacket interface.
func (i *AdventureSettings) Read(buf *bytes.Buffer) {
	i.Flags = ReadInt(buf)
}

// Write implements MCPEPacket interface.
func (i AdventureSettings) Write() *bytes.Buffer {
	buf := new(bytes.Buffer)
	WriteInt(buf, i.Flags)
	return buf
}

// BlockEntityData needs to be documented.
type BlockEntityData struct {
	X        uint32
	Y        uint32
	Z        uint32
	NamedTag []byte
}

// Pid implements MCPEPacket interface.
func (i BlockEntityData) Pid() byte { return BlockEntityDataHead }

// Read implements MCPEPacket interface.
func (i *BlockEntityData) Read(buf *bytes.Buffer) {
	i.X = ReadInt(buf)
	i.Y = ReadInt(buf)
	i.Z = ReadInt(buf)
	i.NamedTag = buf.Bytes()
}

// Write implements MCPEPacket interface.
func (i BlockEntityData) Write() *bytes.Buffer {
	buf := new(bytes.Buffer)
	WriteInt(buf, i.X)
	WriteInt(buf, i.Y)
	WriteInt(buf, i.Z)
	buf.Write(i.NamedTag)
	return buf
}

// Packet-specific constants
const (
	OrderColumns byte = 0
	OrderLayered byte = 1
)

// FullChunkData needs to be documented.
type FullChunkData struct {
	ChunkX, ChunkZ uint32
	Order          byte
	Payload        []byte
}

// Pid implements MCPEPacket interface.
func (i FullChunkData) Pid() byte { return FullChunkDataHead }

// Read implements MCPEPacket interface.
func (i *FullChunkData) Read(buf *bytes.Buffer) {
	BatchRead(buf, &i.ChunkX, &i.ChunkZ, &i.Order)
	i.Payload = buf.Next(int(ReadInt(buf)))
}

// Write implements MCPEPacket interface.
func (i FullChunkData) Write() *bytes.Buffer {
	buf := new(bytes.Buffer)
	BatchWrite(buf, i.ChunkX, i.ChunkZ, i.Order,
		uint32(len(i.Payload)), i.Payload)
	return buf
}

// SetDifficulty needs to be documented.
type SetDifficulty struct {
	Difficulty uint32
}

// Pid implements MCPEPacket interface.
func (i SetDifficulty) Pid() byte { return SetDifficultyHead }

// Read implements MCPEPacket interface.
func (i *SetDifficulty) Read(buf *bytes.Buffer) {
	i.Difficulty = ReadInt(buf)
}

// Write implements MCPEPacket interface.
func (i SetDifficulty) Write() *bytes.Buffer {
	buf := new(bytes.Buffer)
	WriteInt(buf, i.Difficulty)
	return buf
}

// SetPlayerGametype needs to be documented.
type SetPlayerGametype struct {
	Gamemode uint32
}

// Pid implements MCPEPacket interface.
func (i SetPlayerGametype) Pid() byte { return SetPlayerGametypeHead }

// Read implements MCPEPacket interface.
func (i *SetPlayerGametype) Read(buf *bytes.Buffer) {
	i.Gamemode = ReadInt(buf)
}

// Write implements MCPEPacket interface.
func (i SetPlayerGametype) Write() *bytes.Buffer {
	buf := new(bytes.Buffer)
	WriteInt(buf, i.Gamemode)
	return buf
}

// PlayerListEntry needs to be documented.
type PlayerListEntry struct {
	RawUUID            [16]byte
	EntityID           uint64
	Username, Skinname string
	Skin               []byte
}

// Packet-specific constants
const (
	PlayerListRemove byte = 0 // UUID only
	PlayerListAdd    byte = 1 // Everything!
)

// PlayerList needs to be documented.
type PlayerList struct {
	Type          byte
	PlayerEntries []PlayerListEntry
}

// Pid implements MCPEPacket interface.
func (i PlayerList) Pid() byte { return PlayerListHead }

// Read implements MCPEPacket interface.
func (i *PlayerList) Read(buf *bytes.Buffer) {
	i.Type = ReadByte(buf)
	entryCnt := ReadInt(buf)
	i.PlayerEntries = make([]PlayerListEntry, entryCnt)
	for k := uint32(0); k < entryCnt; k++ {
		entry := PlayerListEntry{}
		copy(entry.RawUUID[:], buf.Next(16))
		if i.Type == PlayerListRemove {
			i.PlayerEntries[k] = entry
			continue
		}
		entry.EntityID = ReadLong(buf)
		entry.Username = ReadString(buf)
		entry.Skinname = ReadString(buf)
		entry.Skin = []byte(ReadString(buf))
		i.PlayerEntries[k] = entry
	}
}

// Write implements MCPEPacket interface.
func (i PlayerList) Write() *bytes.Buffer {
	buf := new(bytes.Buffer)
	WriteByte(buf, i.Type)
	WriteInt(buf, uint32(len(i.PlayerEntries)))
	for _, entry := range i.PlayerEntries {
		buf.Write(entry.RawUUID[:])
		if i.Type == PlayerListRemove {
			continue
		}
		WriteLong(buf, entry.EntityID)
		WriteString(buf, entry.Username)
		WriteString(buf, entry.Skinname)
		WriteShort(buf, uint16(len(entry.Skin)))
		Write(buf, entry.Skin)
	}
	return buf
}

// RequestChunkRadius needs to be documented.
type RequestChunkRadius struct {
	Radius uint32
}

// Pid implements MCPEPacket interface.
func (i RequestChunkRadius) Pid() byte { return RequestChunkRadiusHead }

// Read implements MCPEPacket interface.
func (i *RequestChunkRadius) Read(buf *bytes.Buffer) {
	i.Radius = ReadInt(buf)
}

// Write implements MCPEPacket interface.
func (i RequestChunkRadius) Write() *bytes.Buffer {
	buf := new(bytes.Buffer)
	WriteInt(buf, i.Radius)
	return buf
}

// ChunkRadiusUpdate needs to be documented.
type ChunkRadiusUpdate struct {
	Radius uint32
}

// Pid implements MCPEPacket interface.
func (i ChunkRadiusUpdate) Pid() byte { return ChunkRadiusUpdateHead }

// Read implements MCPEPacket interface.
func (i *ChunkRadiusUpdate) Read(buf *bytes.Buffer) {
	i.Radius = ReadInt(buf)
}

// Write implements MCPEPacket interface.
func (i ChunkRadiusUpdate) Write() *bytes.Buffer {
	buf := new(bytes.Buffer)
	WriteInt(buf, i.Radius)
	return buf
}
