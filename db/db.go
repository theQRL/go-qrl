package db

import (
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/filter"
	"github.com/cyyber/go-QRL/log"
	"sync"
)

type LDB struct {
	db *leveldb.DB

	filename string

	exitLock sync.Mutex

	log log.Logger
}

type ldbBatch struct {
	db   *leveldb.DB
	b    *leveldb.Batch
	size int
}

func NewDB(file string, cache int, handles int, log *log.Logger) (*LDB, error) {
	if cache < 16 {
		cache = 16
	}
	if handles < 16 {
		handles = 16
	}
	db, err := leveldb.OpenFile(file, &opt.Options{
		OpenFilesCacheCapacity: handles,
		BlockCacheCapacity:     cache / 2 * opt.MiB,
		WriteBuffer:            cache / 4 * opt.MiB,
		Filter:                 filter.NewBloomFilter(10),
	})

	if err != nil {
		return nil, err
	}

	return &LDB{
		filename: file,
		db:       db,
		log:      log,
	}, nil
}

func (db *LDB) Put(key []byte, value[] byte) error {
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