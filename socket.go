package radv

import (
	"context"
	"net"
	"net/netip"
	"os"
	"time"

	"github.com/mdlayher/ndp"
	"golang.org/x/net/ipv6"
)

const (
	maxRecvSize = 1500
)

// rAdvSocket is a raw socket for sending RA and receiving RS
type rAdvSocket interface {
	hardwareAddr() net.HardwareAddr
	sendRA(ctx context.Context, dst netip.Addr, msg *ndp.RouterAdvertisement) error
	recvRS(ctx context.Context) (*ndp.RouterSolicitation, netip.Addr, error)
	close()
}

type rAdvSocketCtor func(string) (rAdvSocket, error)

// A real socket
type sock struct {
	conn  *ndp.Conn
	iface *net.Interface
}

var _ rAdvSocket = &sock{}

func newRAdvSocket(ifaceName string) (rAdvSocket, error) {
	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		return nil, err
	}
	conn, _, err := ndp.Listen(iface, ndp.LinkLocal)
	if err != nil {
		return nil, err
	}
	return &sock{conn: conn, iface: iface}, nil
}

func (s *sock) hardwareAddr() net.HardwareAddr {
	return s.iface.HardwareAddr
}

func (s *sock) sendRA(ctx context.Context, addr netip.Addr, msg *ndp.RouterAdvertisement) error {
	var err error

	ch := make(chan any)

	go func() {
		defer close(ch)
		// Write to the raw socket shouldn't take long. 2 seconds is long
		// enough time that indicates something wrong happening.
		s.conn.SetWriteDeadline(time.Now().Add(time.Second * 2))
		err = s.conn.WriteTo(msg, nil, addr)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-ch:
	}

	return err
}

func (s *sock) recvRS(ctx context.Context) (*ndp.RouterSolicitation, netip.Addr, error) {
	var (
		m    ndp.Message
		from netip.Addr
		err  error
	)

	ch := make(chan any)

	go func() {
		defer close(ch)
		for {
			// Set read deadline to avoid blocking forever. If there's any way
			// to cancel the read operation, it would be better.
			s.conn.SetReadDeadline(time.Now().Add(time.Millisecond * 500))

			m, _, from, err = s.conn.ReadFrom()
			if err != nil {
				if os.IsTimeout(err) {
					continue
				}
				return
			}

			if m.Type() != ipv6.ICMPTypeRouterSolicitation {
				// Ignore non-RS message and retry
				continue
			}

			return
		}
	}()

	select {
	case <-ctx.Done():
		return nil, netip.Addr{}, ctx.Err()
	case <-ch:
	}

	if err != nil {
		return nil, netip.Addr{}, err
	}

	return m.(*ndp.RouterSolicitation), from, nil
}

func (s *sock) close() {
	s.conn.Close()
}
