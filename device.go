package ra

import (
	"context"
	"net"

	"github.com/vishvananda/netlink"
)

type deviceState struct {
	isUp             bool
	v6LLAddrAssigned bool
	addr             net.HardwareAddr
}

type deviceWatcher interface {
	watch(ctx context.Context, name string) (<-chan deviceState, error)
}

type netlinkDeviceWatcher struct{}

var _ deviceWatcher = &netlinkDeviceWatcher{}

func newDeviceWatcher() deviceWatcher {
	return &netlinkDeviceWatcher{}
}

func (w *netlinkDeviceWatcher) watch(ctx context.Context, name string) (<-chan deviceState, error) {
	linkCh := make(chan netlink.LinkUpdate)
	addrCh := make(chan netlink.AddrUpdate)

	if err := netlink.LinkSubscribeWithOptions(
		linkCh,
		ctx.Done(),
		netlink.LinkSubscribeOptions{
			ErrorCallback: func(err error) {},
			ListExisting:  true,
		},
	); err != nil {
		return nil, err
	}

	if err := netlink.AddrSubscribeWithOptions(
		addrCh,
		ctx.Done(),
		netlink.AddrSubscribeOptions{
			ErrorCallback: func(err error) {},
			ListExisting:  true,
		},
	); err != nil {
		return nil, err
	}

	devCh := make(chan deviceState)

	go func() {
		currentState := deviceState{}
		for {
			select {
			case <-ctx.Done():
				return
			case link := <-linkCh:
				if link.Attrs().Name != name {
					continue
				}
				currentState.isUp = link.Flags&uint32(net.FlagUp) != 0
				currentState.addr = link.Attrs().HardwareAddr
				devCh <- currentState
			case addr := <-addrCh:
				iface, err := net.InterfaceByIndex(addr.LinkIndex)
				if err != nil {
					continue
				}
				if iface.Name != name {
					continue
				}
				if !addr.LinkAddress.IP.IsLinkLocalUnicast() {
					continue
				}
				if addr.NewAddr {
					currentState.v6LLAddrAssigned = true
				} else {
					currentState.v6LLAddrAssigned = false
				}
				devCh <- currentState
			}
		}
	}()

	return devCh, nil
}
