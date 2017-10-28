package anet

var (
	EthernetHeaderSize = 14
)

// EthernetHeader provides getters and setters for fields in an ethernet
// header.
type EthernetHeader []byte

func (e EthernetHeader) Destination() []byte {
	return e[:6]
}

func (e EthernetHeader) Source() []byte {
	return e[6:12]
}

func (e EthernetHeader) Type() uint16 {
	return uint16(e[12])<<8 | uint16(e[13])
}

func (e EthernetHeader) SetType(t uint16) {
	e[12] = byte(t >> 8)
	e[13] = byte(t)
}
