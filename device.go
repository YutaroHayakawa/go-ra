package ra

import (
	"context"
	"net"

	"github.com/vishvananda/netlink"
)

type deviceState struct {
	isUp bool
	addr net.HardwareAddr
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

	devCh := make(chan deviceState)

	go func() {
		defer close(linkCh)
		defer close(devCh)
		for {
			select {
			case <-ctx.Done():
				return
			case link := <-linkCh:
				if link.Attrs().Name != name {
					continue
				}
				devCh <- deviceState{
					isUp: link.Flags&uint32(net.FlagUp) != 0,
					addr: link.Attrs().HardwareAddr,
				}
			}
		}
	}()

	return devCh, nil
}
