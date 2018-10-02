package p2p

import (
	"time"

	"github.com/theQRL/go-qrl/generated"
)

type Msg struct {
	msg			*generated.LegacyMessage
	ReceivedAt	time.Time
}
