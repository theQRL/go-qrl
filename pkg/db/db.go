package db

import (
	"path"
	"sync"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"

	"github.com/theQRL/go-qrl/pkg/log"
)

type LDB struct {
	db *leveldb.DB

	filename string

	exitLock sync.Mutex

	log log.LoggerInterface
}

type ldbBatch struct {
	db   *leveldb.DB
	b    *leveldb.Batch
	size int
}

func NewDB(directory string, filename string, cache int, handles int) (*LDB, error) {
	dbDir := path.Join(directory, filename)
	if cache < 16 {
		cache = 16
	}
	if handles < 16 {
		handles = 16
	}
	db, err := leveldb.OpenFile(dbDir, &opt.Options{
		OpenFilesCacheCapacity: handles,
		BlockCacheCapacity:     cache / 2 * opt.MiB,
		WriteBuffer:            cache / 4 * opt.MiB,
		//Filter:                 filter.NewBloomFilter(10), Bloom filter disabled
	})

	if err != nil {
		return nil, err
	}

	return &LDB{
		filename: filename,
		db:       db,
		log:      log.GetLogger(),
	}, nil
}

func (db *LDB) Put(key []byte, value []byte, batch *leveldb.Batch) error {
	if batch != nil {
		batch.Put(key, value)
		return nil
	}
	return db.db.Put(key, value, nil)
}

func (db *LDB) Get(key []byte) ([]byte, error) {
	value, err := db.db.Get(key, nil)
	if err != nil {
		return nil, err
	}
	return value, err
}

func (db *LDB) Delete(key []byte) error {
	return db.db.Delete(key, nil)
}

func (db *LDB) Close() {
	db.exitLock.Lock()
	defer db.exitLock.Unlock()

	err := db.db.Close()
	if err == nil {
		db.log.Info("LevelDB Closed")
	} else {
		db.log.Error("Failed to close LevelDB", "err", err)
	}

}

func (db *LDB) LDB() *leveldb.DB {
	return db.db
}

func (db *LDB) GetBatch() *leveldb.Batch {
	return &leveldb.Batch{}
}

func (db *LDB) WriteBatch(batch *leveldb.Batch, sync bool) error {
	var wo *opt.WriteOptions
	if sync {
		wo = &opt.WriteOptions{Sync: sync}
	}
	return db.db.Write(batch, wo)
}

func (db *LDB) NewBatch() *ldbBatch {
	return &ldbBatch{db: db.db, b: new(leveldb.Batch)}
}

func (b *ldbBatch) Put(key, value []byte) error {
	b.b.Put(key, value)
	b.size += len(value)
	return nil
}

func (b *ldbBatch) Write() error {
	return b.db.Write(b.b, nil)
}

func (b *ldbBatch) ValueSize() int {
	return b.size
}

func (b *ldbBatch) Reset() {
	b.b.Reset()
	b.size = 0
}
