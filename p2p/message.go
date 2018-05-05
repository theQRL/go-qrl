package p2p

import (
	"github.com/cyyber/go-QRL/generated"
	"time"
)

type Msg struct {
	msg			generated.LegacyMessage
	ReceivedAt	time.Time
}
