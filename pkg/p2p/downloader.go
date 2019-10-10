package p2p

import (
	"github.com/theQRL/go-qrl/pkg/core/block"
	"github.com/theQRL/go-qrl/pkg/core/chain"
	"github.com/theQRL/go-qrl/pkg/generated"
	"github.com/theQRL/go-qrl/pkg/log"
	"github.com/theQRL/go-qrl/pkg/misc"
	"github.com/theQRL/go-qrl/pkg/ntp"
	"math/big"
	"math/rand"
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
	headerHashes             [][]byte
	lastRequestedBlockNumber uint64

	requestedBlockNumbers    map[uint64] bool
}

func (t *TargetNode) AddRequestedBlockNumbers(blockNumber uint64) {
	t.requestedBlockNumbers[blockNumber] = false
}

func (t *TargetNode) RemoveRequestedBlockNumber(blockNumber uint64) bool {
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
	ntp        ntp.NTPInterface

	blockAndPeerChannel chan *BlockAndPeer

	wg                        sync.WaitGroup
	exitDownloadMonitor       chan struct{}
	peersList                 map[string]*Peer
	targetPeers               map[string]*TargetNode
	targetPeerList            []string
	peerSelectionCount        int
	nextConsumableBlockNumber uint64 // Next Block Number that could be consumed by consumer and added into chain
	blockNumberProcessed      chan uint64
	done                      chan struct{}
	consumerRunning           bool
	blockDownloaderRunning    bool
}

func (d *Downloader) AddPeer(p *Peer) {
	d.lock.Lock()
	defer d.lock.Unlock()

	if _, ok := d.peersList[p.ID()]; ok {
		return
	}

	d.peersList[p.ID()] = p
}

func (d *Downloader) RemovePeer(p *Peer) bool {
	d.lock.Lock()
	defer d.lock.Unlock()

	if _, ok := d.peersList[p.ID()]; !ok {
		return false
	}

	delete(d.peersList, p.ID())
	return true
}

func (d *Downloader) GetTargetPeerCount() int {
	d.lock.Lock()
	defer d.lock.Unlock()

	return len(d.targetPeers)
}

func (d *Downloader) GetTargetPeerByID(id string) (*TargetNode, bool) {
	d.lock.Lock()
	defer d.lock.Unlock()

	data, ok := d.targetPeers[id]
	return data, ok
}

func (d *Downloader) GetTargetPeerLength() int {
	d.lock.Lock()
	defer d.lock.Unlock()

	return len(d.targetPeers)
}

func (d *Downloader) GetRandomTargetPeer() *TargetNode {
	d.lock.Lock()
	defer d.lock.Unlock()

	length := len(d.targetPeers)
	if length == 0 {
		return nil
	}

	randIndex := rand.Intn(length)

	for _, targetPeer := range d.targetPeers {
		if randIndex == 0 {
			return targetPeer
		}
		randIndex--
	}

	return nil
}

func (d *Downloader) isDownloaderRunning() bool {
	return d.consumerRunning || d.blockDownloaderRunning
}

func (d *Downloader) AddPeerToTargetPeers(p *Peer) {
	d.lock.Lock()
	defer d.lock.Unlock()

	if _, ok := d.targetPeers[p.ID()]; ok {
		return
	}

	d.targetPeers[p.ID()] = &TargetNode{
		peer: p,
		requestedBlockNumbers: make(map[uint64]bool),
	}
	d.targetPeerList = append(d.targetPeerList, p.ID())
}

func (d *Downloader) RemovePeerFromTargetPeers(p *Peer) bool {
	d.lock.Lock()
	defer d.lock.Unlock()

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
	return false
}

func (d *Downloader) Consumer() {
	d.consumerRunning = true
	defer func () {
		d.consumerRunning = false
	}()
	pendingBlocks := make(map[uint64]*block.Block)
	for {
		select {
		case blockAndPeer := <-d.blockAndPeerChannel:
			b := blockAndPeer.block
			// Ensure if the block received is from the same Peer from which it was requested
			targetPeer, ok := d.GetTargetPeerByID(blockAndPeer.peer.ID())
			if !ok || !targetPeer.IsRequestedBlockNumber(b.BlockNumber()) {
				continue
			}
			// Remove Block Number from Requested list, as it has been received
			targetPeer.RemoveRequestedBlockNumber(b.BlockNumber())

			if b.BlockNumber() < d.nextConsumableBlockNumber {
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
					break
				}

				parentMetaData, err := d.chain.GetBlockMetaData(b.PrevHeaderHash())
				if err != nil {
					d.log.Warn("Impossible Error : ParentMetaData Not Found")
					break
				}

				blockFromState, _ := d.chain.GetBlock(b.HeaderHash())
				measurement, err := d.chain.GetMeasurement(uint32(b.Timestamp()), b.PrevHeaderHash(), parentMetaData)
				if !b.Validate(blockFromState, parentBlock, parentMetaData, measurement, nil) {
					d.log.Warn("Syncing Failed: Block Validation Failed",
						"BlockNumber", b.BlockNumber(),
						"Headerhash", misc.Bin2HStr(b.HeaderHash()))
					break
				}

				if !d.chain.AddBlock(b) {
					d.log.Warn("Failed To Add Block")
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
		case <-time.After(30*time.Second):
			d.log.Info("[Consumer Timeout] Finishing downloading")
			d.isSyncingFinished(true)
			return
		case <-d.done:
			d.isSyncing = false
			return
		}
	}
}

func (d *Downloader) RequestForBlock(blockNumber uint64) error {
	d.log.Info("Requesting For Block")
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
		d.peerSelectionCount = (d.peerSelectionCount + 1) % d.GetTargetPeerLength()
		key := d.targetPeerList[d.peerSelectionCount]
		targetNode, ok := d.GetTargetPeerByID(key)
		if !ok {
			continue
		}
		peerCumulativeDifficultyBytes := targetNode.peer.GetCumulativeDifficulty()
		if peerCumulativeDifficultyBytes == nil {
			d.log.Info("Removing as peerCumulativeDifficultyBytes is nil",
				"peer", targetNode.peer.ID(),
				"Peer count", len(d.targetPeerList))
			d.RemovePeerFromTargetPeers(targetNode.peer)
			continue
		}
		peerCumulativeDifficulty := big.NewInt(0).SetBytes(peerCumulativeDifficultyBytes)
		if peerCumulativeDifficulty.Cmp(localCumulativeDifficulty) <= 0 {
			d.log.Info("Removing as peerCumulativeDifficulty is less than local difficulty",
				"peer", targetNode.peer.ID(),
				"Peer count", len(d.targetPeerList))
			d.RemovePeerFromTargetPeers(targetNode.peer)
			continue
		}
		err := targetNode.peer.SendFetchBlock(blockNumber)
		if err != nil {
			d.log.Info("Removing due to Block fetch error",
				"peer", targetNode.peer.ID(),
				"Peer count", len(d.targetPeerList))
			d.RemovePeerFromTargetPeers(targetNode.peer)
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
	d.blockDownloaderRunning = true
	defer func () {
		d.blockDownloaderRunning = false
	}()

	d.log.Info("Block Downloader Started")

	requestedBlockNumbers :=  make(map[uint64]bool)

	maxRequestedBlockNumber := d.targetNode.lastRequestedBlockNumber
	requestedBlockNumbers[maxRequestedBlockNumber] = false

	err := d.RequestForBlock(maxRequestedBlockNumber)
	if err != nil {
		d.log.Error("Error while Requesting for block",
			"Block #", requestedBlockNumbers[0],
			"Error", err.Error())
	}

	for {
		select {
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
		case <-d.done:
			d.isSyncing = false
			d.log.Info("Producer Exits")
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
	d.lock.Lock()
	defer d.lock.Unlock()

	if !d.isSyncing {
		return true
	}
	lastBlockNumber := d.targetNode.blockNumber + uint64(len(d.targetNode.headerHashes) - 1)
	if d.nextConsumableBlockNumber > lastBlockNumber || forceFinish {
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
		ntp: ntp.GetNTP(),

		peersList: make(map[string]*Peer),
		targetPeers: make(map[string]*TargetNode),
		targetPeerList: make([]string, 0),
		peerSelectionCount: 0,
		blockNumberProcessed: make(chan uint64, SIZE),
		blockAndPeerChannel: make(chan *BlockAndPeer, SIZE*2),
		done: make(chan struct{}),
	}
	return
}

func (d *Downloader) Exit() {
	d.log.Debug("Shutting Down Downloader")
	close(d.exitDownloadMonitor)
	d.wg.Wait()
}

func (d *Downloader) DownloadMonitor() {
	d.log.Info(">>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>> Running download monitor")
	d.exitDownloadMonitor = make(chan struct{})
	d.wg.Add(1)
	defer d.wg.Done()
	for {
		select {
		case <-time.After(30 * time.Second):
			for _, p := range d.peersList {
				// check if peer is already exists in targetPeersList
				if _, ok := d.GetTargetPeerByID(p.ID()); ok {
					continue
				}
				if p.IsPeerAhead() {
					nodeHeaderHashWithTimestamp := p.GetNodeHeaderHashWithTimestamp()
					if nodeHeaderHashWithTimestamp == nil {
						continue
					}
					// Add Peer to targetPeers, if NodeHeaderHash received in last 60 seconds
					if d.ntp.Time() - nodeHeaderHashWithTimestamp.timestamp < 60 {
						d.AddPeerToTargetPeers(p)
					}
				}
			}
			if d.GetTargetPeerCount() == 0 {
				continue
			}
			// Ignore if Consumer or BlockDownloader already running.
			if d.isDownloaderRunning() {
				continue
			}
			if d.isSyncing {
				continue
			}
			// Randomly selects a peer from target peer
			targetNode := d.GetRandomTargetPeer()
			if targetNode == nil {
				d.log.Info("Impossible error: RandomTargetPeer Found Nil")
				continue
			}
			d.log.Info("===================== downloading ====================")
			d.NewTargetNode(targetNode.peer.GetNodeHeaderHashWithTimestamp().nodeHeaderHash, targetNode.peer)
			d.Initialize(targetNode.peer)
		case <-d.exitDownloadMonitor:
			return
		}
	}
}

func (d *Downloader) Initialize(p *Peer) {
	d.log.Info("Initializing Downloader")
	d.isSyncing = true
	d.done = make(chan struct{})
	//d.AddPeer(p)

	nodeHeaderHash := d.targetNode.nodeHeaderHash
	currIndex := d.targetNode.lastRequestedBlockNumber - nodeHeaderHash.BlockNumber

	blockHeaderHash := d.targetNode.nodeHeaderHash.Headerhashes[currIndex]
	_, err := d.chain.GetBlock(blockHeaderHash)
	// Check HeaderHashes that already exists in our blockchain.
	// So that downloading starts from the missing block headerhash.
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
