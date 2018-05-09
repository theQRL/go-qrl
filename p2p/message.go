package p2p

import (
	"github.com/cyyber/go-qrl/generated"
	"time"
)

type Msg struct {
	msg			*generated.LegacyMessage
	ReceivedAt	time.Time
}
