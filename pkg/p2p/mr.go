package p2p

import (
	"github.com/deckarep/golang-set"
	"github.com/theQRL/go-qrl/pkg/config"
	"github.com/theQRL/go-qrl/pkg/core/block"
	"github.com/theQRL/go-qrl/pkg/core/transactions"
	"github.com/theQRL/go-qrl/pkg/generated"
	"github.com/theQRL/go-qrl/pkg/log"
	"github.com/theQRL/go-qrl/pkg/misc"
	"reflect"
	"sync"
)


type OrderedMap struct {
	mapping map[string]*MessageRequest
	order   []string
}

func (o *OrderedMap) Put(k string, v *MessageRequest) {
	o.mapping[k] = v
	o.order = append(o.order, k)
}

func (o *OrderedMap) Get(k string) *MessageRequest {
	return o.mapping[k]
}

func (o *OrderedMap) Delete(k string) {
	delete(o.mapping, k)
	o.order = o.order[1:]  // TODO: Need Optimization
}

type MessageReceipt struct {
	lock sync.Mutex

	allowedTypes mapset.Set
	servicesArgs *map[generated.LegacyMessage_FuncName]string

	hashMsg      map[string]*generated.Message
	hashMsgOrder []string

	requestedHash      map[string]*MessageRequest
	requestedHashOrder []string

	c   *config.Config
	log *log.Logger
}

func (mr *MessageReceipt) addPeer(mrData *generated.MRData, peer *Peer) {
	mr.lock.Lock()
	defer mr.lock.Unlock()

	msgType := mrData.Type
	msgHash := misc.Bin2HStr(mrData.Hash)
	if !mr.allowedTypes.Contains(msgType) {
		return
	}

	if len(mr.requestedHashOrder) >= int(mr.c.Dev.MessageQSize) {
		delete(mr.requestedHash, mr.requestedHashOrder[0])
		mr.requestedHashOrder = mr.requestedHashOrder[1:]
	}

	if _, ok := mr.requestedHash[msgHash]; !ok {
		mr.requestedHash[msgHash] = CreateMessageRequest(mrData, peer)
		mr.requestedHashOrder = append(mr.requestedHashOrder, msgHash)
	} else {
		mr.requestedHash[msgHash].addPeer(peer)
	}
}

func (mr *MessageReceipt) IsRequested(msgHashBytes []byte, peer *Peer, block *block.Block) bool {
	mr.lock.Lock()
	defer mr.lock.Unlock()

	msgHash := misc.Bin2HStr(msgHashBytes)
	if requestedHash, ok := mr.requestedHash[msgHash]; ok {
		if _, ok := requestedHash.peers[peer]; ok {
			return true
		}
	}

	if block != nil {
		if mr.blockParams(msgHash, block) {
			return true
		}
	}

	mr.removePeerFromRequestedHash(msgHash, peer)
	return false
}

func (mr *MessageReceipt) blockParams(msgHash string, block *block.Block) bool {
	requestedHash, ok := mr.requestedHash[msgHash]
	if !ok {
		return false
	}
	coinbaseTx := transactions.ProtoToTransaction(block.Transactions()[0])

	if !reflect.DeepEqual(coinbaseTx.AddrFrom(), requestedHash.mrData.StakeSelector) {
		return false
	}

	if !reflect.DeepEqual(block.PrevHeaderHash(), requestedHash.mrData.PrevHeaderhash) {
		return false
	}

	return true
}

func (mr *MessageReceipt) removePeerFromRequestedHash(msgHash string, peer *Peer) {
	if messageRequest, ok := mr.requestedHash[msgHash]; ok {
		if _, ok := messageRequest.peers[peer]; ok {
			delete(messageRequest.peers, peer)
			if len(messageRequest.peers) == 0 {
				mr.removeRequestedHash(msgHash)
			}
		}
	}
}

func (mr *MessageReceipt) removeRequestedHash(msgHash string) {
	delete(mr.requestedHash, msgHash)
	for index, hash := range mr.requestedHashOrder {
		if hash == msgHash {
			mr.requestedHashOrder = append(mr.requestedHashOrder[:index], mr.requestedHashOrder[index+1:]...)
			break
		}
	}
}

func (mr *MessageReceipt) RemoveRequestedHash(msgHash string) {
	mr.lock.Lock()
	defer mr.lock.Unlock()

	mr.removeRequestedHash(msgHash)
}

func (mr *MessageReceipt) contains(msgHashBytes []byte, messageType generated.LegacyMessage_FuncName) bool {
	mr.lock.Lock()
	defer mr.lock.Unlock()

	msgHash := misc.Bin2HStr(msgHashBytes)
	value, ok := mr.hashMsg[msgHash]
	if !ok {
		return false
	}

	return value.MessageType == messageType
}

func (mr *MessageReceipt) Get(messageType *generated.LegacyMessage_FuncName, messageHash []byte) *generated.LegacyMessage {
	mr.lock.Lock()
	defer mr.lock.Unlock()

	msgHash := misc.Bin2HStr(messageHash)
	value, ok := mr.hashMsg[msgHash]

	if !ok {
		return nil
	}

	switch value.MessageType {
	case generated.LegacyMessage_VE:
		return &generated.LegacyMessage {
			FuncName: *messageType,
			Data: value.Msg,
		}
	case generated.LegacyMessage_PL:
		return &generated.LegacyMessage {
			FuncName: *messageType,
			Data: value.Msg,
		}
	case generated.LegacyMessage_PONG:
		return &generated.LegacyMessage {
			FuncName: *messageType,
			Data: value.Msg,
		}
	case generated.LegacyMessage_MR:
		return &generated.LegacyMessage {
			FuncName: *messageType,
			Data: value.Msg,
		}
	case generated.LegacyMessage_SFM:
		return &generated.LegacyMessage {
			FuncName: *messageType,
			Data: value.Msg,
		}
	case generated.LegacyMessage_BK:
		return &generated.LegacyMessage {
			FuncName: *messageType,
			Data: value.Msg,
		}
	case generated.LegacyMessage_FB:
		return &generated.LegacyMessage {
			FuncName: *messageType,
			Data: value.Msg,
		}
	case generated.LegacyMessage_PB:
		return &generated.LegacyMessage {
			FuncName: *messageType,
			Data: value.Msg,
		}
	case generated.LegacyMessage_TX:
		return &generated.LegacyMessage {
			FuncName: *messageType,
			Data: value.Msg,
		}
	case generated.LegacyMessage_MT:
		return &generated.LegacyMessage {
			FuncName: *messageType,
			Data: value.Msg,
		}
	case generated.LegacyMessage_TK:
		return &generated.LegacyMessage {
			FuncName: *messageType,
			Data: value.Msg,
		}
	case generated.LegacyMessage_TT:
		return &generated.LegacyMessage {
			FuncName: *messageType,
			Data: value.Msg,
		}
	case generated.LegacyMessage_LT:
		return &generated.LegacyMessage {
			FuncName: *messageType,
			Data: value.Msg,
		}
	case generated.LegacyMessage_SL:
		return &generated.LegacyMessage {
			FuncName: *messageType,
			Data: value.Msg,
		}
	case generated.LegacyMessage_EPH:
		return &generated.LegacyMessage {
			FuncName: *messageType,
			Data: value.Msg,
		}
	case generated.LegacyMessage_SYNC:
		return &generated.LegacyMessage {
			FuncName: *messageType,
			Data: value.Msg,
		}
	default:
		return nil
	}
}

func (mr *MessageReceipt) GetHashMsg(msgHash string) (value *generated.Message, ok bool) {
	mr.lock.Lock()
	defer mr.lock.Unlock()

	value, ok = mr.hashMsg[msgHash]
	return
}

func (mr *MessageReceipt) GetRequestedHash(msgHash string) (value *MessageRequest, ok bool) {
	mr.lock.Lock()
	defer mr.lock.Unlock()

	value, ok = mr.requestedHash[msgHash]
	return
}

func CreateMR() (mr *MessageReceipt) {

	allowedTypes := mapset.NewSet()
	allowedTypes.Add(generated.LegacyMessage_TX)
	allowedTypes.Add(generated.LegacyMessage_LT)
	allowedTypes.Add(generated.LegacyMessage_EPH)
	allowedTypes.Add(generated.LegacyMessage_BK)
	allowedTypes.Add(generated.LegacyMessage_MT)
	allowedTypes.Add(generated.LegacyMessage_TK)
	allowedTypes.Add(generated.LegacyMessage_TT)
	allowedTypes.Add(generated.LegacyMessage_SL)

	servicesArgs := &map[generated.LegacyMessage_FuncName] string {
		generated.LegacyMessage_VE: "veData",
		generated.LegacyMessage_PL: "plData",
		generated.LegacyMessage_PONG: "pongData",

		generated.LegacyMessage_MR: "mrData",
		generated.LegacyMessage_SFM: "mrData",

		generated.LegacyMessage_BK: "block",
		generated.LegacyMessage_FB: "fbData",
		generated.LegacyMessage_PB: "pbData",

		generated.LegacyMessage_TX: "txData",
		generated.LegacyMessage_MT: "mtData",
		generated.LegacyMessage_TK: "tkData",
		generated.LegacyMessage_TT: "ttData",
		generated.LegacyMessage_LT: "ltData",
		generated.LegacyMessage_SL: "slData",

		generated.LegacyMessage_EPH: "ephData",

		generated.LegacyMessage_SYNC: "syncData",
	}

	mr = &MessageReceipt {
		allowedTypes: allowedTypes,
		hashMsg: make(map[string]*generated.Message),
		hashMsgOrder: make([]string, 1),
		requestedHash: make(map[string]*MessageRequest),
		requestedHashOrder: make([]string, 1),
		servicesArgs: servicesArgs,
		c: config.GetConfig(),
		log: log.GetLogger(),
	}

	return
}
