package ra

import "context"

type fakeDeviceWatcher struct {
	watchers map[string]chan deviceState
}

var _ deviceWatcher = &fakeDeviceWatcher{}

func newFakeDeviceWatcher(devs ...string) *fakeDeviceWatcher {
	fdw := &fakeDeviceWatcher{
		watchers: make(map[string]chan deviceState),
	}
	for _, dev := range devs {
		fdw.watchers[dev] = make(chan deviceState, 1)
	}
	return fdw
}

func (w *fakeDeviceWatcher) watch(ctx context.Context, name string) (<-chan deviceState, error) {
	devCh := make(chan deviceState)

	go func() {
		defer close(devCh)
		for {
			select {
			case <-ctx.Done():
				return
			case dev := <-w.watchers[name]:
				devCh <- dev
			}
		}
	}()

	return devCh, nil
}

func (w *fakeDeviceWatcher) update(name string, dev deviceState) {
	w.watchers[name] <- dev
}
