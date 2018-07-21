package misc

import (
	"github.com/beevik/ntp"
	"time"
	"sync"
	"github.com/cyyber/go-qrl/core"
)

type NTP struct {
	lock *sync.Mutex

	drift uint64
	lastUpdate uint64
	config *core.Config
}

func (n *NTP) UpdateTime() error {
	n.lock.Lock()
	defer n.lock.Unlock()

	var err error
	var t time.Time

	for retry := 0; retry <= n.config.User.NTP.Retries; retry++ {
		for _, server := range n.config.User.NTP.Servers {
			t, err = ntp.Time(server)

			if err != nil {
				continue
			}

			n.drift = uint64(time.Now().Second() - t.Second())
			n.lastUpdate = uint64(t.Second())

			return nil
		}
	}

	return err
}

func (n *NTP) Time() uint64 {
	currentTime := uint64(time.Now().Second()) + n.drift
	if currentTime - n.lastUpdate > n.config.User.NTP.Refresh {
		err := n.UpdateTime()
		if err != nil {
			// TODO: log warning here
		}
	}

	return uint64(time.Now().Second()) + n.drift
}

var once sync.Once
var n *NTP

func GetNTP() *NTP {
	once.Do(func() {
		n = &NTP{}
	})

	return n
}
