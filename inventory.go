package highmc

// Inventory is just a set of items, for containers or inventory holder entities.
type Inventory []Item

// PlayerInventory is a inventory holder for players.
type PlayerInventory struct {
	*Inventory
	Hotbars []Item
	Hand    Item
	Holder  *player
}

// Init initializes the inventory.
func (pi *PlayerInventory) Init() {
	pi.Hotbars = make([]Item, 8)
	if true { // No survival inventory now
		inv := make(Inventory, len(CreativeItems))
		copy(inv, CreativeItems)
		pi.Inventory = &inv
		pi.Holder.SendCompressed(&ContainerSetContent{
			WindowID: CreativeWindow,
			Slots:    inv,
		})
	}
}
