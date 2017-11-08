package anet

import (
	"bytes"
	"fmt"

	"github.com/google/netstack/tcpip"
	"github.com/google/netstack/tcpip/buffer"
	"github.com/google/netstack/tcpip/stack"
)

const (
	NETIF_RSP_NULL = 1
)

// LinkEndpoint wraps a low-level xen network device to implement netstack's
// stack.LinkEndpoint interface.
type LinkEndpoint struct {
	device     *Device
	mac        tcpip.LinkAddress
	dispatcher stack.NetworkDispatcher
}

func NewLinkEndpoint(dev *Device) *LinkEndpoint {
	return &LinkEndpoint{
		device: dev,
		mac:    parseHardwareAddr(string(dev.MacAddr)),
	}
}

func parseHardwareAddr(s string) tcpip.LinkAddress {
	var addr [6]byte

	fmt.Sscanf(
		s,
		"%02x:%02x:%02x:%02x:%02x:%02x",
		&addr[0],
		&addr[1],
		&addr[2],
		&addr[3],
		&addr[4],
		&addr[5],
	)

	return tcpip.LinkAddress(addr[:])
}

func (e LinkEndpoint) MTU() uint32                    { return 1500 }
func (e LinkEndpoint) MaxHeaderLength() uint16        { return uint16(EthernetHeaderSize) }
func (e LinkEndpoint) LinkAddress() tcpip.LinkAddress { return e.mac }

// WritePacket implements stack.LinkEndpoint
func (e *LinkEndpoint) WritePacket(r *stack.Route, hdr *buffer.Prependable, payload buffer.View, protocol tcpip.NetworkProtocolNumber) *tcpip.Error {
	if r.RemoteLinkAddress == "" {
		// TODO: I think we would want to save this packet,
		// call LinkAddressRequest, and then deliver after the address
		// is resolved. But this part of the arp handling is also TODO
		// in netstack, so we'll just drop it.
		println("anet: dropping packet with missing remote link addr")
		return nil
	}

	ethhdr := EthernetHeader(hdr.Prepend(EthernetHeaderSize))
	copy(ethhdr.Destination(), r.RemoteLinkAddress)
	copy(ethhdr.Source(), e.mac)
	ethhdr.SetType(uint16(protocol))

	e.writeEthernetPacket(hdr, payload)
	return nil
}

// writeEthernetPacket writes hdr and payload to a tx buffer
// and delivers it to the front-end driver to send.
func (e *LinkEndpoint) writeEthernetPacket(hdr *buffer.Prependable, payload buffer.View) {
	buf, _ := e.device.TxBuffers.Get()

	w := bytes.NewBuffer(buf.Page.Data[:0])
	w.Write(hdr.UsedBytes())
	w.Write(payload)

	e.device.SendTxBuffer(buf, w.Len())

	if notify := e.device.Tx.PushRequests(); notify {
		e.device.EventChannel.Notify()
	}
}

// Attach sets dispatcher as the target for network packet delivery
// and starts receiving packets.
//
// Attach implements stack.LinkEndpoint
func (e *LinkEndpoint) Attach(dispatcher stack.NetworkDispatcher) {
	e.dispatcher = dispatcher

	go e.rxLoop()
}

// rxLoop receives rx buffers from the front-end
// and delivers network packets to the attached dispatcher.
func (e *LinkEndpoint) rxLoop() {
	var dev = e.device

	for {
		dev.EventChannel.Wait()

		for dev.Rx.CheckForResponses() {
			var (
				rsp = (*NetifRxResponse)(dev.Rx.NextResponse())
				buf = dev.RxBuffers.Lookup(int(rsp.ID))
			)

			e.deliverPacket(rsp, buf)

			dev.SendRxBuffer(buf)
		}

		if notify := dev.Rx.PushRequests(); notify {
			dev.EventChannel.Notify()
		}
	}
}

// deliverPacket handles the network packet in buf and delivers it
// to the attached network dispatcher.
func (e *LinkEndpoint) deliverPacket(rsp *NetifRxResponse, buf *Buffer) {
	if rsp.Status <= NETIF_RSP_NULL {
		return
	}

	size := int(rsp.Status)
	view := buffer.NewView(size)
	copy(view, buf.Page.Data[rsp.Offset:])

	ethhdr := EthernetHeader(view[:EthernetHeaderSize])
	view.TrimFront(EthernetHeaderSize)

	vv := buffer.NewVectorisedView(len(view), []buffer.View{view})

	e.dispatcher.DeliverNetworkPacket(
		e,
		tcpip.LinkAddress(ethhdr.Source()),
		tcpip.NetworkProtocolNumber(ethhdr.Type()),
		&vv,
	)
}
