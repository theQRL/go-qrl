package p2p

import (
	"io"
	"time"
)

type Msg struct {
	Code       uint64
	Size       uint32 // size of the payload
	Payload    io.Reader
	ReceivedAt time.Time
}
