package p2p

import (
	"github.com/theQRL/go-qrl/pkg/generated"
	"sync"
)

type MessageRequest struct {
	lock      sync.Mutex
	peers     map[*Peer]bool
	mrData    *generated.MRData
	requested bool // True if Request for full message has already been done from the peer
}

func (messageRequest *MessageRequest) addPeer(peer *Peer) {
	messageRequest.lock.Lock()
	defer messageRequest.lock.Unlock()

	messageRequest.peers[peer] = false
}

func (messageRequest *MessageRequest) SetPeer(peer *Peer, value bool) {
	messageRequest.lock.Lock()
	defer messageRequest.lock.Unlock()

	messageRequest.peers[peer] = value
}

func (messageRequest *MessageRequest) SetRequested(value bool) {
	messageRequest.lock.Lock()
	defer messageRequest.lock.Unlock()

	messageRequest.requested = value
}

func (messageRequest *MessageRequest) GetRequested() bool {
	messageRequest.lock.Lock()
	defer messageRequest.lock.Unlock()

	return messageRequest.requested
}

func CreateMessageRequest(mrData *generated.MRData, peer *Peer) (messageRequest *MessageRequest) {
	messageRequest = &MessageRequest {
		peers: make(map[*Peer]bool),
		mrData: mrData,
		requested: false,
	}
	messageRequest.peers[peer] = false
	return
}