package db

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/theQRL/go-qrl/pkg/log"
)

type TestLDB struct {
	ldb *LDB
	tempDir string
}

func NewTestLDB() *TestLDB {
	tempDir, err := ioutil.TempDir("", "")
	logger := log.New()
	ldb, err := NewDB(tempDir, "state",16, 16, &logger)

	if err != nil {
		panic(err)
	}

	return &TestLDB{ldb, tempDir}
}

func (db *TestLDB) Close() {
	defer os.RemoveAll(db.tempDir)
	db.ldb.Close()
}

func TestPut(t *testing.T) {
	db := NewTestLDB()
	defer db.Close()

	key := []byte("key1")
	value := []byte("value1")

	err := db.ldb.Put(key, value, nil)
	assert.Nil(t, err, "%v", err)
}

func TestGet(t *testing.T) {
	db := NewTestLDB()
	defer db.Close()

	key := []byte("key1")
	value := []byte("value1")

	err := db.ldb.Put(key, value, nil)
	assert.Nil(t, err, "%v", err)

	v, err := db.ldb.Get(key)
	assert.Equal(t, value, v, "%v != %v", value, v)
}

func TestGetBatch(t *testing.T) {
	db := NewTestLDB()
	defer db.Close()

	batch := db.ldb.GetBatch()
	assert.NotNil(t, batch, "Batch is nil")
}

func TestWriteBatch(t *testing.T) {
	db := NewTestLDB()
	defer db.Close()

	batch := db.ldb.GetBatch()

	key := []byte("key1")
	value := []byte("value1")

	err := db.ldb.Put(key, value, batch)
	assert.Nil(t, err, "%v", err)

	// Should not get the value as batch has not been written
	v, err := db.ldb.Get(key)
	assert.Error(t, err, "Expected error but value found %s", value)

	err = db.ldb.WriteBatch(batch, true)
	assert.Nil(t, err, "%v", err)

	v, err = db.ldb.Get(key)
	assert.Nil(t, err, "%v", err)
	assert.Equal(t, value, v, "%v != %v", value, v)
}

func TestNewBatch(t *testing.T) {
	db := NewTestLDB()
	defer db.Close()

	ldbBatch := db.ldb.NewBatch()
	assert.NotNil(t, ldbBatch, "ldbBatch nil")
}

func TestLdbBatch_Put(t *testing.T) {
	db := NewTestLDB()
	defer db.Close()

	ldbBatch := db.ldb.NewBatch()
	assert.NotNil(t, ldbBatch, "ldbBatch nil")

	key := []byte("key1")
	value := []byte("value1")

	err := ldbBatch.Put(key, value)
	assert.Nil(t, err, "%v", err)

	assert.Equal(t, len(value), ldbBatch.size, "Mismatch %v != %v", len(value), ldbBatch.size)

	v, err := db.ldb.Get(key)
	assert.Error(t, err, "Expected error but value found %s", v)
}

func TestLdbBatch_Write(t *testing.T) {
	db := NewTestLDB()
	defer db.Close()

	ldbBatch := db.ldb.NewBatch()
	assert.NotNil(t, ldbBatch, "ldbBatch nil")

	key := []byte("key1")
	value := []byte("value1")

	err := ldbBatch.Put(key, value)
	assert.Nil(t, err, "%v", err)

	assert.Equal(t, len(value), ldbBatch.size, "ldbBatch size mismatch")

	v, err := db.ldb.Get(key)
	assert.Error(t, err, "Expected error but value found %s", v)

	ldbBatch.Write()
	v, err = db.ldb.Get(key)
	assert.Nil(t, err, "%v", err)
	assert.Equal(t, value, v, "%v != %v", value, v)
}

func TestLdbBatch_ValueSize(t *testing.T) {
	db := NewTestLDB()
	defer db.Close()

	ldbBatch := db.ldb.NewBatch()
	assert.NotNil(t, ldbBatch, "ldbBatch nil")

	assert.Equal(t, ldbBatch.size, 0, "Mismatch %v != %v", ldbBatch.size, 0)
	assert.Equal(t, ldbBatch.size, ldbBatch.ValueSize(), "Mismatch %v != %v", ldbBatch.size, ldbBatch.ValueSize())

	key := []byte("key1")
	value := []byte("value1")

	err := ldbBatch.Put(key, value)
	assert.Nil(t, err, "%v", err)

	assert.Equal(t, len(value), ldbBatch.size, "Mismatch %v != %v", len(value), ldbBatch.size)
	assert.Equal(t, ldbBatch.size, ldbBatch.ValueSize(), "Mismatch %v != %v", ldbBatch.size, ldbBatch.ValueSize())
}

func TestLdbBatch_Reset(t *testing.T) {
	db := NewTestLDB()
	defer db.Close()

	ldbBatch := db.ldb.NewBatch()
	assert.NotNil(t, ldbBatch, "ldbBatch nil")

	assert.Equal(t, ldbBatch.size, 0, "Mismatch %v != %v", ldbBatch.size, 0)
	assert.Equal(t, ldbBatch.size, ldbBatch.ValueSize(), "Mismatch %v != %v", ldbBatch.size, ldbBatch.ValueSize())

	key := []byte("key1")
	value := []byte("value1")

	err := ldbBatch.Put(key, value)
	assert.Nil(t, err, "%v", err)

	assert.Equal(t, len(value), ldbBatch.size, "Mismatch %v != %v", len(value), ldbBatch.size)
	assert.Equal(t, ldbBatch.size, ldbBatch.ValueSize(), "Mismatch %v != %v", ldbBatch.size, ldbBatch.ValueSize())

	ldbBatch.Reset()
	assert.Equal(t, ldbBatch.size, 0, "Mismatch %v != %v", ldbBatch.size, 0)
	assert.Equal(t, ldbBatch.size, ldbBatch.ValueSize(), "Mismatch %v != %v", ldbBatch.size, ldbBatch.ValueSize())
}
