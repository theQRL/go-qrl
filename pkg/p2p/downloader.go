package p2p

import (
	"github.com/theQRL/go-qrl/pkg/core/block"
	"github.com/theQRL/go-qrl/pkg/core/chain"
	"github.com/theQRL/go-qrl/pkg/generated"
	"github.com/theQRL/go-qrl/pkg/log"
	"github.com/theQRL/go-qrl/pkg/misc"
	"github.com/theQRL/qrllib/goqrllib/goqrllib"
	"reflect"
	"sync"
	"time"
)

type BlockAndPeer struct {
	block *block.Block
	peer *Peer
}

type NodeHeaderHashAndPeer struct {
	nodeHeaderHash *generated.NodeHeaderHash
	peer *Peer
}

type TargetNode struct {
	blockNumber              uint64
	nodeHeaderHash			 *generated.NodeHeaderHash
	targetHeaderHash		 []byte
	headerHashes             [][]byte
	peer                     *Peer
	lastRequestedBlockNumber uint64
	retry                    uint16
}

type Downloader struct {
	lock sync.Mutex

	lastPBTime uint64
	targetNode *TargetNode
	isSyncing  bool
	chain      *chain.Chain
	log        *log.Logger

	blockAndPeerChannel chan *BlockAndPeer
}

func (d *Downloader) NewTargetNode(nodeHeaderHash *generated.NodeHeaderHash, peer *Peer) {
	d.targetNode = &TargetNode {
		blockNumber: nodeHeaderHash.BlockNumber,
		headerHashes: nodeHeaderHash.Headerhashes,
		nodeHeaderHash: nodeHeaderHash,
		lastRequestedBlockNumber: nodeHeaderHash.BlockNumber,
		peer: peer,
	}
}

func (d *Downloader) isSyncingFinished(forceFinish bool) bool {
	currIndex := d.targetNode.lastRequestedBlockNumber - d.targetNode.blockNumber + 1
	if int(currIndex) == len(d.targetNode.headerHashes) || forceFinish {
		d.isSyncing = false
		d.targetNode = &TargetNode{}
		return true
	}
	return false
}

func (d *Downloader) peerFetchBlock() bool {
	nodeHeaderHash := d.targetNode.nodeHeaderHash
	currIndex := d.targetNode.lastRequestedBlockNumber - nodeHeaderHash.BlockNumber

	blockHeaderHash := d.targetNode.nodeHeaderHash.Headerhashes[currIndex]
	b, err := d.chain.GetBlock(blockHeaderHash)

	for ; err == nil && int(currIndex+1) < len(d.targetNode.nodeHeaderHash.Headerhashes); {
		d.targetNode.lastRequestedBlockNumber += 1
		currIndex = d.targetNode.lastRequestedBlockNumber - nodeHeaderHash.BlockNumber
		blockHeaderHash = d.targetNode.nodeHeaderHash.Headerhashes[currIndex]
		b, err = d.chain.GetBlock(blockHeaderHash)
	}

	if b != nil && d.isSyncingFinished(false) {
		d.log.Info(">>Syncing Finished<<")
		return false
	}

	d.targetNode.peer.SendFetchBlock(d.targetNode.lastRequestedBlockNumber)
	d.targetNode.retry += 1

	return true
}

func (d *Downloader) BlockDownloader() {
	d.log.Info("Block Downloader Started",
	"peer", d.targetNode.peer.conn.RemoteAddr().String())
	d.isSyncing = true
	defer d.isSyncingFinished(true)
	d.peerFetchBlock()
	for {
		select {
		case blockAndPeer := <-d.blockAndPeerChannel:
			peer := blockAndPeer.peer
			b := blockAndPeer.block

			if peer != d.targetNode.peer {
				if d.targetNode.peer == nil {
					d.log.Warn("Received block and target peer is None")
				} else {
					d.log.Warn("Received block from unexpected peer",
						"Expected peer", d.targetNode.peer.conn.RemoteAddr().String(),
						"Found peer", peer)
				}
				return
			}

			if b.BlockNumber() != d.targetNode.lastRequestedBlockNumber {
				d.log.Warn("Received Block Number doesnt match",
					"Last Requested Block Number", d.targetNode.lastRequestedBlockNumber,
					"Received Block Number", b.BlockNumber())
				return
			}

			targetStartBlockNumber := d.targetNode.blockNumber
			expectedHeaderHash := d.targetNode.headerHashes[b.BlockNumber()-targetStartBlockNumber]
			if !reflect.DeepEqual(b.HeaderHash(), expectedHeaderHash) {
				d.log.Warn("Did not match headerhash",
					"Expected headerhash", goqrllib.Bin2hstr(expectedHeaderHash),
					"Found headerhash", goqrllib.Bin2hstr(b.HeaderHash()))
				return
			}

			b2, _ := d.chain.GetBlock(b.HeaderHash())
			parentBlock, err := d.chain.GetBlock(b.PrevHeaderHash())
			if err != nil {
				d.log.Warn("Parent Block Not Found")
				return
			}

			parentMetaData, err := d.chain.GetBlockMetaData(b.PrevHeaderHash())
			if err != nil {
				d.log.Warn("Impossible Error : ParentMetaData Not Found")
				return
			}

			measurement, err := d.chain.GetMeasurement(uint32(b.Timestamp()), b.PrevHeaderHash(), parentMetaData)
			if !b.Validate(b2, parentBlock, parentMetaData, measurement, nil) {
				d.log.Warn("Syncing Failed: Block Validation Failed",
					"BlockNumber", b.BlockNumber(),
					"Headerhash", misc.Bin2HStr(b.HeaderHash()))
				return
			}

			if !d.chain.AddBlock(b) {
				d.log.Warn("Failed To Add Block")
				return
			}

			if !reflect.DeepEqual(d.chain.GetLastBlock().HeaderHash(), b.HeaderHash()) {
				// TODO: Suspend Mining Timestamp, while successfully downloading blocks
				// self.pow.suspend_mining_timestamp = ntp.getTime() + config.dev.sync_delay_mining
				d.log.Info("Block Added",
					"BlockNumber", b.BlockNumber(),
					"Headerhash", b.HeaderHash())
			}

			if d.isSyncingFinished(false) {
				d.log.Info("Block Download Syncing Finished")
				return
			}

			d.targetNode.lastRequestedBlockNumber += 1
			d.targetNode.retry = 0

			if !d.peerFetchBlock() {
				d.log.Info("~~~~~~~~~~~~~Ended Downloader~~~~~~~~~")
				return
			}

		case <-time.After(100*time.Second):
			d.log.Warn("Block Downloading Timeout")
			if d.targetNode.retry >= 1 {
				d.log.Warn("Retry Limit Hit")
				// TODO: Ban Peer
				d.isSyncingFinished(true)
				return
			}
			if !d.peerFetchBlock() {
				return
			}
		}
	}
}

func CreateDownloader(c *chain.Chain) (d *Downloader) {
	d = &Downloader {
		lastPBTime: 0,
		targetNode: nil,
		isSyncing: false,
		chain: c,
		log: log.GetLogger(),
		blockAndPeerChannel: make(chan *BlockAndPeer),
	}
	return
}
