package p2p

import (
	"github.com/theQRL/go-qrl/pkg/core/block"
	"github.com/theQRL/go-qrl/pkg/core/chain"
	"github.com/theQRL/go-qrl/pkg/generated"
	"github.com/theQRL/go-qrl/pkg/log"
	"github.com/theQRL/go-qrl/pkg/misc"
	"math/big"
	"reflect"
	"sync"
	"time"
)

const SIZE = 40 // Size has to be calculated based on maximum possible values on Queue

type BlockAndPeer struct {
	block *block.Block
	peer *Peer
}

type NodeHeaderHashAndPeer struct {
	nodeHeaderHash *generated.NodeHeaderHash
	peer *Peer
}

type TargetNode struct {
	peer                     *Peer

	blockNumber              uint64
	nodeHeaderHash			 *generated.NodeHeaderHash
	targetHeaderHash		 []byte
	headerHashes             [][]byte
	lastRequestedBlockNumber uint64
	retry                    uint16


	requestedBlockNumbers    map[uint64] bool // TODO: make map Memory Initialization
}

func (t *TargetNode) AddRequestedBlockNumbers(blockNumber uint64) {
	t.requestedBlockNumbers[blockNumber] = false
}

func (t *TargetNode) RemoveRequestedBlockNumbers(blockNumber uint64) bool {
	if _, ok := t.requestedBlockNumbers[blockNumber]; !ok {
		return false
	}
	delete(t.requestedBlockNumbers, blockNumber)
	return true
}

func (t *TargetNode) IsRequestedBlockNumber(blockNumber uint64) bool {
	_, ok := t.requestedBlockNumbers[blockNumber]
	return ok
}

type Downloader struct {
	lock sync.Mutex

	targetNode *TargetNode
	isSyncing  bool
	chain      *chain.Chain
	log        *log.Logger

	blockAndPeerChannel chan *BlockAndPeer

	targetPeers               map[string]*TargetNode
	targetPeerList            []string
	peerSelectionCount        int
	nextConsumableBlockNumber uint64 // Next Block Number that could be consumed by consumer and added into chain
	blockNumberReceived       chan uint64
	blockNumberProcessed      chan uint64
	retryBlockNumber          chan uint64
	done                      chan struct{}
}

func (d *Downloader) AddPeer(p *Peer) {
	d.lock.Lock()
	defer d.lock.Unlock()

	if !d.isSyncing {
		return
	}

	if _, ok := d.targetPeers[p.ID()]; ok {
		p.SetDownloaderPeerList(true)
		return
	}

	d.targetPeers[p.ID()] = &TargetNode{
		peer:p,
		requestedBlockNumbers: make(map[uint64]bool),
	}
	d.targetPeerList = append(d.targetPeerList, p.ID())
	p.SetDownloaderPeerList(true)
}

func (d *Downloader) removePeer(p *Peer) bool {
	if !d.isSyncing {
		return false
	}

	if _, ok := d.targetPeers[p.ID()]; !ok {
		return false
	}

	delete(d.targetPeers, p.ID())
	for i, targetPeerID := range d.targetPeerList {
		if targetPeerID == p.ID() {
			d.targetPeerList = append(d.targetPeerList[:i], d.targetPeerList[i+1:]...)
			return true
		}
	}
	p.SetDownloaderPeerList(false)
	return false
}

func (d *Downloader) Consumer() {
	pendingBlocks := make(map[uint64]*block.Block)
	for {
		select {
		case blockAndPeer := <-d.blockAndPeerChannel:
			b := blockAndPeer.block
			d.lock.Lock()

			// Ensure if the block received is from the same Peer from which it was requested
			if targetPeer, ok := d.targetPeers[blockAndPeer.peer.ID()]; ok {
				if !targetPeer.IsRequestedBlockNumber(b.BlockNumber()) {
					d.lock.Unlock()
					continue
				}
			} else {
				d.lock.Unlock()
				continue
			}
			// Remove Block Number from Requested list, as it has been received
			d.targetPeers[blockAndPeer.peer.ID()].RemoveRequestedBlockNumbers(b.BlockNumber())
			d.lock.Unlock()

			d.blockNumberReceived <- b.BlockNumber()  // Notify Downloader about the received BlockNumber

			if b.BlockNumber() < d.nextConsumableBlockNumber {
				d.blockNumberProcessed <- b.BlockNumber()  // Block Received which has already been processed
				continue
			}
			pendingBlocks[b.BlockNumber()] = b

			for d.isSyncing {
				b, ok := pendingBlocks[d.nextConsumableBlockNumber]
				if !ok {
					break
				}
				d.log.Info("Trying To Add Block",
					"#", b.BlockNumber(),
					"headerhash", misc.Bin2HStr(b.HeaderHash()))
				delete(pendingBlocks, d.nextConsumableBlockNumber)
				parentBlock, err := d.chain.GetBlock(b.PrevHeaderHash())
				if err != nil {
					// TODO: Think of how to handle such cases
					d.log.Warn("Parent Block Not Found")
					d.retryBlockNumber <- b.BlockNumber()
					break
				}

				parentMetaData, err := d.chain.GetBlockMetaData(b.PrevHeaderHash())
				if err != nil {
					d.log.Warn("Impossible Error : ParentMetaData Not Found")
					d.retryBlockNumber <- b.BlockNumber()
					break
				}

				blockFromState, _ := d.chain.GetBlock(b.HeaderHash())
				measurement, err := d.chain.GetMeasurement(uint32(b.Timestamp()), b.PrevHeaderHash(), parentMetaData)
				if !b.Validate(blockFromState, parentBlock, parentMetaData, measurement, nil) {
					d.log.Warn("Syncing Failed: Block Validation Failed",
						"BlockNumber", b.BlockNumber(),
						"Headerhash", misc.Bin2HStr(b.HeaderHash()))
					d.retryBlockNumber <- b.BlockNumber()
					break
				}

				if !d.chain.AddBlock(b) {
					d.log.Warn("Failed To Add Block")
					d.retryBlockNumber <- b.BlockNumber()
					break
				}

				if !reflect.DeepEqual(d.chain.GetLastBlock().HeaderHash(), b.HeaderHash()) {
					// TODO: Suspend Mining Timestamp, while successfully downloading blocks
					// self.pow.suspend_mining_timestamp = ntp.getTime() + config.dev.sync_delay_mining
					d.log.Info("Block Added",
						"BlockNumber", b.BlockNumber(),
						"Headerhash", misc.Bin2HStr(b.HeaderHash()))
				}
				d.blockNumberProcessed <- b.BlockNumber()
				d.nextConsumableBlockNumber++
			}

			if d.isSyncingFinished(false) {
				d.log.Info("Block Download Syncing Finished")
				return
			}
		case <-d.done:
			d.isSyncing = false
			return
		}
	}
}

func (d *Downloader) RequestForBlock(blockNumber uint64) error {
	d.lock.Lock()
	defer d.lock.Unlock()
	d.log.Info("Requesting For Blocks",
		"peers", len(d.targetPeerList))
	lastBlock := d.chain.GetLastBlock()
	blockMetaData, err := d.chain.GetBlockMetaData(lastBlock.HeaderHash())
	if err != nil {
		d.log.Error("Error fetching Block Metadata",
			"BlockHeaderHash", misc.Bin2HStr(lastBlock.HeaderHash()),
			"Error", err.Error())
		return err
	}

	localCumulativeDifficulty := big.NewInt(0).SetBytes(blockMetaData.TotalDifficulty())
	for len(d.targetPeerList) > 0 {
		// TODO: Replace sequential selection by number of pending requests
		d.peerSelectionCount = (d.peerSelectionCount + 1) % len(d.targetPeers)
		key := d.targetPeerList[d.peerSelectionCount]
		targetNode := d.targetPeers[key]
		peerCumulativeDifficultyBytes := targetNode.peer.GetCumulativeDifficulty()
		if peerCumulativeDifficultyBytes == nil {
			d.log.Info("Removing as peerCumulativeDifficultyBytes is nil",
				"peer", targetNode.peer.ID(),
				"Peer count", len(d.targetPeerList))
			d.removePeer(targetNode.peer)
			continue
		}
		peerCumulativeDifficulty := big.NewInt(0).SetBytes(peerCumulativeDifficultyBytes)
		if peerCumulativeDifficulty.Cmp(localCumulativeDifficulty) <= 0 {
			d.log.Info("Removing as peerCumulativeDifficulty is less than local difficulty",
				"peer", targetNode.peer.ID(),
				"Peer count", len(d.targetPeerList))
			d.removePeer(targetNode.peer)
			continue
		}
		err := targetNode.peer.SendFetchBlock(blockNumber)
		if err != nil {
			d.log.Info("Removing due to Block fetch error",
				"peer", targetNode.peer.ID(),
				"Peer count", len(d.targetPeerList))
			d.removePeer(targetNode.peer)
			continue
		}
		targetNode.AddRequestedBlockNumbers(blockNumber)
		break
	}
	if len(d.targetPeerList) == 0 {
		d.isSyncingFinished(true)
	}
	return nil
}

func (d *Downloader) BlockDownloader() {
	requestedBlockNumbers :=  make(map[uint64]bool)
	d.log.Info("Block Downloader Started")
	maxRequestedBlockNumber := d.targetNode.lastRequestedBlockNumber
	requestedBlockNumbers[d.targetNode.lastRequestedBlockNumber] = false
	err := d.RequestForBlock(maxRequestedBlockNumber)
	if err != nil {
		d.log.Error("Error while Requesting for block",
			"Block #", requestedBlockNumbers[0],
			"Error", err.Error())
	}

	for {
		select {
		case blockNumber := <-d.blockNumberReceived:
			requestedBlockNumbers[blockNumber] = true
		case blockNumber := <-d.blockNumberProcessed:
			delete(requestedBlockNumbers, blockNumber)
			for len(requestedBlockNumbers) < SIZE - 10 {
				blockNumber := maxRequestedBlockNumber + 1
				err := d.RequestForBlock(blockNumber)
				if err != nil {
					d.log.Error("Error while Requesting for block",
						"Block #", blockNumber,
						"Error", err.Error())
					continue
				}
				maxRequestedBlockNumber = blockNumber
				requestedBlockNumbers[maxRequestedBlockNumber] = false
			}
		case blockNumber := <-d.retryBlockNumber:
			requestedBlockNumbers[blockNumber] = false
		case <-time.After(10*time.Second):
			d.log.Info("Producer Timeout",
				"len of requestedBlockNumbers", len(requestedBlockNumbers))
			if len(requestedBlockNumbers) == 0 {
				d.blockNumberReceived <- 0
			} else if len(requestedBlockNumbers) >= SIZE - 10 {
				consumeableBlockNumber := d.nextConsumableBlockNumber
				for key := range requestedBlockNumbers {
					if key < consumeableBlockNumber {
						delete(requestedBlockNumbers, key)
					}
				}
			}

			for blockNumber, value := range requestedBlockNumbers {
				if !value {
					err := d.RequestForBlock(blockNumber)
					if err != nil {
						d.log.Error("Error while Requesting for block",
							"Block #", blockNumber,
							"Error", err.Error())
					}
				}
			}
		case <-d.done:
			d.isSyncing = false
			d.log.Info("Producer Exitted")
			return
		}
	}
}

func (d *Downloader) NewTargetNode(nodeHeaderHash *generated.NodeHeaderHash, peer *Peer) {
	d.targetNode = &TargetNode {
		peer: peer,
		blockNumber: nodeHeaderHash.BlockNumber,
		headerHashes: nodeHeaderHash.Headerhashes,
		nodeHeaderHash: nodeHeaderHash,
		lastRequestedBlockNumber: nodeHeaderHash.BlockNumber,

		requestedBlockNumbers: make(map[uint64]bool),
	}
}

func (d *Downloader) isSyncingFinished(forceFinish bool) bool {
	if !d.isSyncing {
		return true
	}
	lastBlockNumber := d.targetNode.blockNumber + uint64(len(d.targetNode.headerHashes) - 1)
	if d.nextConsumableBlockNumber >= lastBlockNumber || forceFinish {
		d.isSyncing = false
		d.targetNode = nil
		d.targetPeers = make(map[string]*TargetNode)
		d.targetPeerList = make([]string, 0)
		close(d.done)
		d.log.Info("Syncing FINISHED")
		return true
	}
	return false
}

func CreateDownloader(c *chain.Chain) (d *Downloader) {
	d = &Downloader {
		targetNode: nil,
		isSyncing: false,
		chain: c,
		log: log.GetLogger(),

		targetPeers: make(map[string]*TargetNode),
		targetPeerList: make([]string, 0),
		peerSelectionCount: 0,
		blockNumberReceived: make(chan uint64, SIZE),
		blockNumberProcessed: make(chan uint64, SIZE),
		retryBlockNumber: make(chan uint64, 1),
		blockAndPeerChannel: make(chan *BlockAndPeer, SIZE*2),
		done: make(chan struct{}),
	}
	return
}

func (d *Downloader) Initialize(p *Peer) {
	d.log.Info("Initializing Downloader")
	d.isSyncing = true
	d.done = make(chan struct{})
	d.AddPeer(p)

	nodeHeaderHash := d.targetNode.nodeHeaderHash
	currIndex := d.targetNode.lastRequestedBlockNumber - nodeHeaderHash.BlockNumber

	blockHeaderHash := d.targetNode.nodeHeaderHash.Headerhashes[currIndex]
	_, err := d.chain.GetBlock(blockHeaderHash)

	for ; err == nil && int(currIndex+1) < len(d.targetNode.nodeHeaderHash.Headerhashes); {
		d.targetNode.lastRequestedBlockNumber += 1
		currIndex = d.targetNode.lastRequestedBlockNumber - nodeHeaderHash.BlockNumber
		blockHeaderHash = d.targetNode.nodeHeaderHash.Headerhashes[currIndex]
		_, err = d.chain.GetBlock(blockHeaderHash)
	}
	d.nextConsumableBlockNumber = d.targetNode.lastRequestedBlockNumber
	go d.Consumer()
	go d.BlockDownloader()
}