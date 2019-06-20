package p2p

import (
	"container/heap"
	"encoding/binary"
	"github.com/golang/protobuf/proto"
	"github.com/theQRL/go-qrl/pkg/generated"
	"github.com/theQRL/go-qrl/pkg/log"
	"github.com/theQRL/go-qrl/pkg/ntp"
)

type OutgoingMessage struct {
	msg          *generated.LegacyMessage
	priority     uint64
	timestamp    uint64
	bytesMessage []byte
	ntp          ntp.NTPInterface
	index        int
}

func (o *OutgoingMessage) IsExpired() bool {
	currTimestamp := o.ntp.Time()
	return currTimestamp - o.timestamp > 90
}

func CreateOutgoingMessage(priority uint64, msg *generated.LegacyMessage) *OutgoingMessage {
	data, err := proto.Marshal(msg)
	if err != nil {
		log.GetLogger().Warn("Error Parsing Data while creating OutgoingMessage")
		log.GetLogger().Info(err.Error())
		return nil
	}
	bs := make([]byte, 4)
	binary.BigEndian.PutUint32(bs, uint32(len(data)))
	out := append(bs, data...)

	o := &OutgoingMessage{
		msg:msg,
		priority:priority,
		timestamp:ntp.GetNTP().Time(),
		bytesMessage: out,
		ntp: ntp.GetNTP(),
	}
	return o
}

type PriorityQueue []*OutgoingMessage

func (pq *PriorityQueue) RemoveExpiredMessages() {
	old := *pq
	for i, o := range *pq {
		if o.IsExpired() {
			*pq = append(old[:i], old[i+1:]...)
		}
	}
}

func (pq PriorityQueue) Full() bool {
	if pq.Len() > 2000 {
		pq.RemoveExpiredMessages()
	}
	return pq.Len() > 2000
}

func (pq PriorityQueue) Len() int {
	return len(pq)
}

func (pq PriorityQueue) Less(i, j int) bool {
	return pq[i].priority < pq[j].priority
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *PriorityQueue) Push(x interface{}) {
	n := len(*pq)
	outgoingMessage := x.(*OutgoingMessage)
	outgoingMessage.index = n
	*pq = append(*pq, outgoingMessage)
}

func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	if n == 0 {
		return nil
	}
	outgoingMessage := old[n-1]
	outgoingMessage.index = -1 // for safety
	*pq = old[:n-1]
	return outgoingMessage
}

func (pq *PriorityQueue) update(o *OutgoingMessage, bytesMessage []byte, priority uint64) {
	o.bytesMessage = bytesMessage
	o.priority = priority
	heap.Fix(pq, o.index)
}
