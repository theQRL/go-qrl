package p2p

import (
	"github.com/stretchr/testify/assert"
	"github.com/theQRL/go-qrl/pkg/generated"
	"testing"
)

type PriorityQueueTest struct {
	pq PriorityQueue
}

func NewPriorityQueueTest() *PriorityQueueTest {
	return &PriorityQueueTest{}
}

func TestPriorityQueue_RemoveExpiredMessages(t *testing.T) {
	pq := NewPriorityQueueTest()
	p2pAck := &generated.P2PAcknowledgement{
		BytesProcessed: 100,
	}
	msg := &generated.LegacyMessage{
		FuncName: generated.LegacyMessage_P2P_ACK,
		Data: &generated.LegacyMessage_P2PAckData{
			P2PAckData: p2pAck,
		},
	}
	o1 := CreateOutgoingMessage(1, msg)
	o2 := CreateOutgoingMessage(1, msg)
	o3 := CreateOutgoingMessage(1, msg)
	assert.NotNil(t, o1)
	assert.NotNil(t, o2)
	assert.NotNil(t, o3)

	pq.pq.Push(o1)
	pq.pq.Push(o2)
	pq.pq.Push(o3)
	assert.Equal(t, pq.pq.Len(), 3)
	pq.pq.RemoveExpiredMessages()
	assert.Equal(t, pq.pq.Len(), 3)
	o2.timestamp = 0
	pq.pq.RemoveExpiredMessages()
	assert.Equal(t, pq.pq.Len(), 2)
}