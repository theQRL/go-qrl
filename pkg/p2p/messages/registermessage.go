package messages

import "github.com/theQRL/go-qrl/pkg/generated"

type RegisterMessage struct {
	MsgHash string
	Msg     *generated.Message
}
