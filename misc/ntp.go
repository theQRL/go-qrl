package misc

import (
	"github.com/beevik/ntp"
	"time"
	"sync"
)

type NTP struct {
	lock *sync.Mutex
	drift uint64
	lastUpdate uint64
}

func (n *NTP) UpdateTime() error {
	n.lock.Lock()
	defer n.lock.Unlock()

	NTP_RETRIES := 6

	var err error
	var t time.Time

	for retry := 0; retry <= NTP_RETRIES; retry++ {
		t, err = ntp.Time("pool.ntp.org")

		if err != nil {
			continue
		}

		n.drift = uint64(time.Now().Second() - t.Second())
		n.lastUpdate = uint64(t.Second())

		return nil
	}

	return err
}

func (n *NTP) Time() (uint64, error) {
	currentTime := uint64(time.Now().Second()) + n.drift
	if currentTime - n.lastUpdate > 12 * 60 * 60 {
		err := n.UpdateTime()
		return 0, err
	}

	return uint64(time.Now().Second()) + n.drift, nil
}

func CreateNTP() (*NTP, error) {
	n := &NTP{}

	err := n.UpdateTime()

	return n, err
}
