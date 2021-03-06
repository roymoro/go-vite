package vitedb

import (
	"math/big"
	"github.com/vitelabs/go-vite/ledger"
	"log"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/util"
	"encoding/hex"
	"fmt"
)

type AccountChain struct {
	db *DataBase
}


var _accountchain *AccountChain

func GetAccountChain () *AccountChain {
	db, err := GetLDBDataBase(DB_BLOCK)
	if err != nil {
		log.Fatal(err)
	}

	if _accountchain == nil {
		_accountchain = &AccountChain{
			db: db,
		}
	}

	return _accountchain
}

func (ac * AccountChain) BatchWrite (batch *leveldb.Batch, writeFunc func (batch *leveldb.Batch) error) error {
	return batchWrite(batch, ac.db.Leveldb, func (context *batchContext) error {
		return writeFunc(context.Batch)
	})
}



func (ac * AccountChain) WriteBlock (batch *leveldb.Batch, accountId *big.Int, accountBlockHeight *big.Int, accountBlock *ledger.AccountBlock) error {
	buf, err :=  accountBlock.DbSerialize()
	if err != nil {
		return err
	}
	key, err := createKey(DBKP_ACCOUNTBLOCK, accountId, accountBlockHeight)
	batch.Put(key, buf)

	return nil
}

func (ac * AccountChain) WriteBlockMeta (batch *leveldb.Batch, accountBlockHash []byte, accountBlockMeta *ledger.AccountBlockMeta) error {
	buf, err :=  accountBlockMeta.DbSerialize()
	if err != nil {
		return err
	}

	key, err := createKey(DBKP_ACCOUNTBLOCKMETA, accountBlockHash)
	batch.Put(key, buf)
	return nil
}


func (ac * AccountChain) GetBlockByHash (blockHash []byte) (*ledger.AccountBlock, error) {
	accountBlockMeta, err := ac.GetBlockMeta(blockHash)
	if err != nil {
		return nil, err
	}

	return ac.GetBlockByHeight(accountBlockMeta.AccountId, accountBlockMeta.Height)
}


func (ac * AccountChain) GetBlockByHeight (accountId *big.Int, blockHeight *big.Int) (*ledger.AccountBlock, error) {
	key, err:= createKey(DBKP_ACCOUNTBLOCK, accountId, blockHeight)
	if err != nil {
		return nil, err
	}

	block, err := ac.db.Leveldb.Get(key, nil)
	if err != nil {
		return nil, err
	}

	accountBlock := &ledger.AccountBlock{}
	accountBlock.DbDeserialize(block)

	accountBlockMeta, err:= ac.GetBlockMeta(accountBlock.Hash)
	if err != nil {
		return nil, err
	}

	accountBlock.Meta = accountBlockMeta

	return accountBlock, nil
}

func (ac *AccountChain) GetLatestBlockByAccountId (accountId *big.Int) (*ledger.AccountBlock, error){

	latestBlockHeight, err := ac.GetLatestBlockHeightByAccountId(accountId)
	fmt.Println(latestBlockHeight.String())

	if err != nil || latestBlockHeight == nil{
		return nil, err
	}

	return ac.GetBlockByHeight(accountId, latestBlockHeight)
}

func (ac *AccountChain) GetLatestBlockHeightByAccountId (accountId *big.Int) (* big.Int, error){
	key, err:= createKey(DBKP_ACCOUNTBLOCK, accountId, nil)
	if err != nil {
		return nil, err
	}

	iter := ac.db.Leveldb.NewIterator(util.BytesPrefix(key), nil)
	defer iter.Release()

	if !iter.Last() {
		fmt.Println("GetLatestBlockHeightByAccountId failed, because account " + accountId.String() + " doesn't exist.")
		return nil, nil
	}

	lastKey := iter.Key()
	partionList := deserializeKey(lastKey)

	latestBlockHeight := &big.Int{}
	latestBlockHeight.SetBytes(partionList[1])
	return latestBlockHeight, nil
}

func (ac *AccountChain) GetBlockListByAccountMeta (index int, num int, count int, meta *ledger.AccountMeta) ([]*ledger.AccountBlock, error) {
	latestBlockHeight, err := ac.GetLatestBlockHeightByAccountId(meta.AccountId)
	if err != nil {
		return nil, err
	}
	limitIndex := latestBlockHeight.Sub(latestBlockHeight, big.NewInt(int64(index * count) - 1))
	limitKey, err := createKey(DBKP_ACCOUNTBLOCK, meta.AccountId, limitIndex)
	if err != nil {
		return nil, err
	}

	startKey, err := createKey(DBKP_ACCOUNTBLOCK, meta.AccountId, big.NewInt(1))
	if err != nil {
		return nil, err
	}

	iter := ac.db.Leveldb.NewIterator(&util.Range{Start: startKey, Limit: limitKey}, nil)
	defer iter.Release()

	if !iter.Last() {
		return nil, nil
	}

	var blockList []*ledger.AccountBlock

	for i:=0; i < num * count; i ++ {
		block := &ledger.AccountBlock{}

		err := block.DbDeserialize(iter.Value())
		if err != nil {
			return nil, err
		}

		blockList = append(blockList, block)

		if !iter.Prev() {
			break
		}
	}

	return blockList, nil
}

func (ac * AccountChain) GetBlockMeta (blockHash []byte) (*ledger.AccountBlockMeta, error) {
	key, err:= createKey(DBKP_ACCOUNTBLOCKMETA, hex.EncodeToString(blockHash))
	if err != nil {
		return nil, err
	}
	blockMetaBytes, err:= ac.db.Leveldb.Get(key, nil)
	if err != nil {
		return nil, err
	}

	blockMeta := &ledger.AccountBlockMeta{}
	if err := blockMeta.DbDeserialize(blockMetaBytes); err != nil {
		return nil, err
	}

	return blockMeta, nil
}

func (ac *AccountChain) WriteStIndex (batch *leveldb.Batch, stHash []byte, id *big.Int, accountBlockHash []byte) error {
	key, err:= createKey(DBKP_SNAPSHOTTIMESTAMP_INDEX, stHash, id)
	if err != nil {
		return err
	}

	batch.Put(key, accountBlockHash)

	return nil
}

// st == SnapshotTimestamp
func (ac *AccountChain) GetLastIdByStHeight (stHeight *big.Int) (*big.Int, error) {
	key, err:= createKey(DBKP_SNAPSHOTTIMESTAMP_INDEX, stHeight)
	if err != nil {
		return nil, err
	}

	iter := ac.db.Leveldb.NewIterator(util.BytesPrefix(key), nil)
	defer iter.Release()
	if !iter.Last() {
		return nil, nil
	}

	lastId := &big.Int{}
	lastId.SetBytes(iter.Value())
	return lastId, nil
}


func (ac *AccountChain) GetBlockHashList (index, num, count int) ([][]byte, error) {
	key, err:= createKey(DBKP_SNAPSHOTTIMESTAMP_INDEX, nil)
	if err != nil {
		return nil, err
	}

	iter := ac.db.Leveldb.NewIterator(util.BytesPrefix(key), nil)
	defer iter.Release()

	if !iter.Last() {
		return nil, nil
	}

	for i:=0; i < index * count; i++ {
		if !iter.Prev() {
			return nil, nil
		}
	}

	var blocHashList [][]byte
	for i:=0; i < num * count; i++ {

		value := make([]byte, len(iter.Value()))
		copy(value, iter.Value())

		blocHashList = append(blocHashList, value)

		if !iter.Prev() {
			break
		}
	}


	return blocHashList, nil
}