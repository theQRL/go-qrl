package ntp

import (
	"sync"
	"time"

	"github.com/beevik/ntp"

	"github.com/theQRL/go-qrl/pkg/config"
)

type NTP struct {
	lock sync.Mutex

	drift      int64
	lastUpdate uint64
	config     *config.Config
}

type NTPInterface interface {
	UpdateTime() error
	Time() uint64
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

			n.drift = int64(time.Now().Unix() - t.Unix())
			n.lastUpdate = uint64(t.Unix())

			return nil
		}
	}

	return err
}

func (n *NTP) Time() uint64 {
	currentTime := uint64(time.Now().Unix() + n.drift)
	if currentTime-n.lastUpdate > n.config.User.NTP.Refresh {
		err := n.UpdateTime()
		if err != nil {
			// TODO: log warning here
		}
	}

	return uint64(time.Now().Unix() + n.drift)
}

var once sync.Once
var n *NTP

func GetNTP() *NTP {
	once.Do(func() {
		n = &NTP{config: config.GetConfig()}
	})

	return n
}
