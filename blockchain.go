package main

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/boltdb/bolt"
)

const dbFile = "blockchain_%s.db"
const blocksBucket = "blocks"
const genesisCoinbaseData = "The Times 03/Jan/2009 Chancellor on brink of second bailout for banks"

// Blockchain implements interactions with a DB
type Blockchain struct {
	//区块链的最后一个区块的哈希
	tip []byte
	//bolt数据库 -  KV数据库，关键字 - 值
	db *bolt.DB
}

// CreateBlockchain creates a new blockchain DB
// 创建一个新的区块链数据库   address:创世区块的地址  nodeID:当且节点ID
func CreateBlockchain(address, nodeID string) *Blockchain {
	dbFile := fmt.Sprintf(dbFile, nodeID)
	if dbExists(dbFile) {
		fmt.Println("Blockchain already exists.")
		os.Exit(1)
	}
	//最新的区块的哈希
	var tip []byte
	//创建创世区块 - 只有输出没有引用任何输入
	cbtx := NewCoinbaseTX(address, genesisCoinbaseData)
	genesis := NewGenesisBlock(cbtx)

	db, err := bolt.Open(dbFile, 0600, nil)
	if err != nil {
		log.Panic(err)
	}

	err = db.Update(func(tx *bolt.Tx) error {
		//创建一个新的桶(数据库)来存储区块中的区块
		b, err := tx.CreateBucket([]byte(blocksBucket))
		if err != nil {
			log.Panic(err)
		}

		//将创世区块放入到数据库中  (区块哈希， 区块序列化后的数据)
		err = b.Put(genesis.Hash, genesis.Serialize())
		if err != nil {
			log.Panic(err)
		}

		//放入最新的区块的哈希([]byte("l")用来维护任何状态下(每次都会变为最新区块的哈希)的最新区块的哈希值)
		err = b.Put([]byte("l"), genesis.Hash)
		if err != nil {
			log.Panic(err)
		}
		tip = genesis.Hash

		return nil
	})
	if err != nil {
		log.Panic(err)
	}

	//赋值好区块链字段后返回
	bc := Blockchain{tip, db}

	return &bc
}

// NewBlockchain creates a new Blockchain with genesis Block
// 根据提供的节点ID来创建一个*Blockchain对象，方便其他地方调用该结点的区块链数据库
// 找到已知的该结点的区块链数据库
func NewBlockchain(nodeID string) *Blockchain {
	dbFile := fmt.Sprintf(dbFile, nodeID)
	if dbExists(dbFile) == false {
		fmt.Println("No existing blockchain found. Create one first.")
		os.Exit(1)
	}

	var tip []byte
	db, err := bolt.Open(dbFile, 0600, nil)
	if err != nil {
		log.Panic(err)
	}

	err = db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
		//通过指定的桶名称获取存储在桶中的最新区块的哈希值，存储在tip
		tip = b.Get([]byte("l"))

		return nil
	})
	if err != nil {
		log.Panic(err)
	}

	//tip最新区块的哈希值
	bc := Blockchain{tip, db}

	return &bc
}

// AddBlock saves the block into the blockchain
func (bc *Blockchain) AddBlock(block *Block) {
	err := bc.db.Update(func(tx *bolt.Tx) error {
		//确定访问bolt下的哪个数据库，访问blocksBucket("block")数据库-相当于表
		b := tx.Bucket([]byte(blocksBucket))
		//查找看有没有这个区块，如果这个区块已经存在则不再添加
		blockInDb := b.Get(block.Hash)

		if blockInDb != nil {
			return nil
		}

		//没有这个区块的话序列化后添加到数据库中
		blockData := block.Serialize()
		err := b.Put(block.Hash, blockData)
		if err != nil {
			log.Panic(err)
		}

		//获取该区块链中的最后一个区块的区块哈希 -> 再得到该区块
		lastHash := b.Get([]byte("l"))
		lastBlockData := b.Get(lastHash)
		lastBlock := DeserializeBlock(lastBlockData)

		//更新tip为新加入的区块
		if block.Height > lastBlock.Height {
			//需要加入新区块，加入新区块后最后一个区块的区块哈希更新为最新
			err = b.Put([]byte("l"), block.Hash)
			if err != nil {
				log.Panic(err)
			}
			bc.tip = block.Hash
		}

		return nil
	})
	if err != nil {
		log.Panic(err)
	}
}

// FindTransaction finds a transaction by its ID
// 查找交易ID是否存在
func (bc *Blockchain) FindTransaction(ID []byte) (Transaction, error) {
	//遍历区块链数据bc的迭代器
	bci := bc.Iterator()

	for {
		block := bci.Next()

		for _, tx := range block.Transactions {
			//找到交易为ID的则返回该交易
			if bytes.Compare(tx.ID, ID) == 0 {
				return *tx, nil
			}
		}

		//如果该区块的前一个区块哈希字段为空，则该区块为创世区块
		if len(block.PrevBlockHash) == 0 {
			break
		}
	}

	return Transaction{}, errors.New("Transaction is not found")
}

// FindUTXO finds all unspent transaction outputs and returns transactions with spent outputs removed
// 查找所有未花费的交易输出，并返回删除了已花费输出的交易
func (bc *Blockchain) FindUTXO() map[string]TXOutputs {
	//为花费交易输出
	UTXO := make(map[string]TXOutputs)
	//已花费交易输出
	spentTXOs := make(map[string][]int)
	bci := bc.Iterator()

	for {
		//获取一个区块
		block := bci.Next()

		//每个区块中遍历所有交易
		//区块中的每个交易的格式
		//交易ID标识，引用的输入，输出
		for _, tx := range block.Transactions {
			txID := hex.EncodeToString(tx.ID)

		Outputs:
			//遍历每花个交易的输出，检查该笔交易的输出是否被费
			for outIdx, out := range tx.Vout {
				// Was the output spent?
				//如果不为空表示该交易的某个输出已经被花费
				if spentTXOs[txID] != nil {
					for _, spentOutIdx := range spentTXOs[txID] {
						if spentOutIdx == outIdx {
							continue Outputs
						}
					}
				}

				//统计所有该txID未被花费的交易
				outs := UTXO[txID]
				outs.Outputs = append(outs.Outputs, out)
				UTXO[txID] = outs
			}

			//如果不是创世区块交易，我们还需要处理交易的输入
			//标记当前交易的输入已经被花费
			if tx.IsCoinbase() == false {
				for _, in := range tx.Vin {
					inTxID := hex.EncodeToString(in.Txid)
					spentTXOs[inTxID] = append(spentTXOs[inTxID], in.Vout)
				}
			}
		}

		//如果当前区块的前一个区块哈希为空，说明已经到了创世区块，所有区块遍历完成
		if len(block.PrevBlockHash) == 0 {
			break
		}
	}

	//返回为花费的交易输出
	return UTXO
}

// Iterator returns a BlockchainIterat
// 创建用来遍历区块链的迭代器
func (bc *Blockchain) Iterator() *BlockchainIterator {
	bci := &BlockchainIterator{bc.tip, bc.db}

	return bci
}

// 返回区块链的高度
// GetBestHeight returns the height of the latest block
func (bc *Blockchain) GetBestHeight() int {
	var lastBlock Block

	err := bc.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
		lastHash := b.Get([]byte("l"))
		blockData := b.Get(lastHash)
		lastBlock = *DeserializeBlock(blockData)

		return nil
	})
	if err != nil {
		log.Panic(err)
	}

	return lastBlock.Height
}

// GetBlock finds a block by its hash and returns it
// 根据区块的哈希值得到一个区块
func (bc *Blockchain) GetBlock(blockHash []byte) (Block, error) {
	var block Block

	err := bc.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))

		blockData := b.Get(blockHash)

		if blockData == nil {
			return errors.New("Block is not found.")
		}

		block = *DeserializeBlock(blockData)

		return nil
	})
	if err != nil {
		return block, err
	}

	return block, nil
}

// GetBlockHashes returns a list of hashes of all the blocks in the chain
// 得到区块链中所有区块的哈希值
func (bc *Blockchain) GetBlockHashes() [][]byte {
	var blocks [][]byte
	bci := bc.Iterator()

	for {
		//利用前一个区块的哈希来遍历整个区块链
		block := bci.Next()

		blocks = append(blocks, block.Hash)

		if len(block.PrevBlockHash) == 0 {
			break
		}
	}

	return blocks
}

// 挖出一个新的区块，并添加到区块链中
// MineBlock mines a new block with the provided transactions
func (bc *Blockchain) MineBlock(transactions []*Transaction) *Block {
	//最后一个区块的哈希值
	var lastHash []byte
	var lastHeight int

	//检验这个交易结构体中的所有交易是否有效
	for _, tx := range transactions {
		// TODO: ignore transaction if it's not valid
		if bc.VerifyTransaction(tx) != true {
			log.Panic("ERROR: Invalid transaction")
		}
	}

	//取出区块中的最后一个块
	err := bc.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
		//读取最后一个区块的哈希值
		lastHash = b.Get([]byte("l"))

		blockData := b.Get(lastHash)
		block := DeserializeBlock(blockData)

		lastHeight = block.Height

		return nil
	})
	if err != nil {
		log.Panic(err)
	}

	//根据交易和最后一个区块的哈希值创建一个新的区块
	newBlock := NewBlock(transactions, lastHash, lastHeight+1)

	//将该块放入到区块链中
	err = bc.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
		err := b.Put(newBlock.Hash, newBlock.Serialize())
		if err != nil {
			log.Panic(err)
		}

		//该处是将最后一个区块的哈希值更新为最新的区块的哈希值
		err = b.Put([]byte("l"), newBlock.Hash)
		if err != nil {
			log.Panic(err)
		}

		//更新最后一个区块的区块哈希
		bc.tip = newBlock.Hash

		return nil
	})
	if err != nil {
		log.Panic(err)
	}

	return newBlock
}

// SignTransaction signs inputs of a Transaction
// 对交易的输入进行签名
func (bc *Blockchain) SignTransaction(tx *Transaction, privKey ecdsa.PrivateKey) {
	//用来存储每个输入引用的 前一个交易 (ID, Tx)
	prevTXs := make(map[string]Transaction)

	for _, vin := range tx.Vin {
		//查找输入引用的交易
		prevTX, err := bc.FindTransaction(vin.Txid)
		if err != nil {
			log.Panic(err)
		}
		prevTXs[hex.EncodeToString(prevTX.ID)] = prevTX
	}

	//对这个块的输入引用进行签名
	tx.Sign(privKey, prevTXs)
}

// VerifyTransaction verifies transaction input signatures
func (bc *Blockchain) VerifyTransaction(tx *Transaction) bool {
	if tx.IsCoinbase() {
		return true
	}

	prevTXs := make(map[string]Transaction)

	for _, vin := range tx.Vin {
		prevTX, err := bc.FindTransaction(vin.Txid)
		if err != nil {
			log.Panic(err)
		}
		prevTXs[hex.EncodeToString(prevTX.ID)] = prevTX
	}

	//验证交易输入的签名是否有效
	return tx.Verify(prevTXs)
}

func dbExists(dbFile string) bool {
	if _, err := os.Stat(dbFile); os.IsNotExist(err) {
		return false
	}

	return true
}
