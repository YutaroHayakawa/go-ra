package radv

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mdlayher/ndp"
)

type fakeSockRegistry struct {
	reg     map[string]*fakeSock
	regLock sync.RWMutex
}

func newFakeSockRegistry() *fakeSockRegistry {
	return &fakeSockRegistry{
		reg: map[string]*fakeSock{},
	}
}

func (r *fakeSockRegistry) newSock(iface string) (rAdvSocket, error) {
	r.regLock.Lock()
	defer r.regLock.Unlock()

	if _, ok := r.reg[iface]; ok {
		return nil, fmt.Errorf("duplicate interface name")
	}

	fs := &fakeSock{
		tx: make(chan fakeRA, 128),
		rx: make(chan fakeRS, 128),
	}
	r.reg[iface] = fs

	return fs, nil
}

func (r *fakeSockRegistry) getSock(iface string) (*fakeSock, error) {
	r.regLock.RLock()
	defer r.regLock.RUnlock()

	fs, ok := r.reg[iface]
	if !ok {
		return nil, fmt.Errorf("interface not found")
	}

	return fs, nil
}

// A fake socket
type fakeSock struct {
	tx     chan fakeRA
	rx     chan fakeRS
	closed atomic.Bool
}

type fakeRA struct {
	tstamp time.Time
	msg    *ndp.RouterAdvertisement
	to     netip.Addr
}

type fakeRS struct {
	msg  *ndp.RouterSolicitation
	from netip.Addr
}

var _ rAdvSocket = &fakeSock{}

func (s *fakeSock) txCh() <-chan fakeRA {
	return s.tx
}

func (s *fakeSock) rxCh() chan<- fakeRS {
	return s.rx
}

func (s *fakeSock) hardwareAddr() net.HardwareAddr {
	return net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}
}

func (s *fakeSock) sendRA(_ context.Context, addr netip.Addr, msg *ndp.RouterAdvertisement) error {
	select {
	case s.tx <- fakeRA{tstamp: time.Now(), msg: msg, to: addr}:
		return nil
	default:
		return fmt.Errorf("tx channel is full")
	}
}

func (s *fakeSock) recvRS(ctx context.Context) (*ndp.RouterSolicitation, netip.Addr, error) {
	select {
	case <-ctx.Done():
		return nil, netip.Addr{}, ctx.Err()
	case rs := <-s.rx:
		return rs.msg, rs.from, nil
	}
}

func (s *fakeSock) close() {
	close(s.tx)
	close(s.rx)
	s.closed.Store(true)
}

func (s *fakeSock) isClosed() bool {
	return s.closed.Load()
}
