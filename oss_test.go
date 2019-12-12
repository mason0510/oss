package main

import (
	"fmt"
	"github.com/syndtr/goleveldb/leveldb"
	"oss/utils"
	"testing"
)

func TestUploadFile(t *testing.T) {

}

func TestDB(t *testing.T) {
	dbConn, _ = leveldb.OpenFile(FileIndexPath, nil)
	defer dbConn.Close()
	iter := dbConn.NewIterator(nil, nil)
	for iter.Next() {
		key := iter.Key()
		value := iter.Value()
		fmt.Println("key: " + utils.Bytes2Str(key))
		fmt.Println("value: " + utils.Bytes2Str(value))
		fmt.Println()
	}
	iter.Release()
	if err := iter.Error(); err != nil {
		fmt.Println(err)
	}
}

func TestSyncFileIndex(t *testing.T) {
	dbConn, _ = leveldb.OpenFile(FileIndexPath, nil)
	defer dbConn.Close()
	SyncFileIndex()
}
