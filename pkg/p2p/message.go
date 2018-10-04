package p2p

import (
	"time"

	"github.com/theQRL/go-qrl/pkg/generated"
)

type Msg struct {
	msg			*generated.LegacyMessage
	ReceivedAt	time.Time
}
