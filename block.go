package main

import (
	"bytes"
	"encoding/gob"
	"log"
	"time"
)

// Block represents a block in the blockchain
type Block struct {
	//区块的创建时间
	Timestamp int64
	//结构体的指针切片，表示区块中的所有交易
	Transactions []*Transaction
	//前一个区块的哈希，每个块都包含前一个区块的哈希，形成链式结构
	PrevBlockHash []byte
	//当前区块的哈希，包含了时间戳、交易、前一个区块的哈希、随机数和高度
	Hash []byte
	//随机数，表示满足工作量证明算法的随机数。通过工作量证明算法的计算得到，使得区块的哈希满足特定的条件
	Nonce int
	//区块在区块链中的高度
	Height int
}

// NewBlock creates and returns Block
func NewBlock(transactions []*Transaction, prevBlockHash []byte, height int) *Block {
	block := &Block{time.Now().Unix(), transactions, prevBlockHash, []byte{}, 0, height}
	//创建工作量证明对象
	pow := NewProofOfWork(block)
	//相当于挖矿，在找合适的nonce
	nonce, hash := pow.Run()

	block.Hash = hash[:]
	block.Nonce = nonce

	return block
}

// NewGenesisBlock creates and returns genesis Block
func NewGenesisBlock(coinbase *Transaction) *Block {
	return NewBlock([]*Transaction{coinbase}, []byte{}, 0)
}

// HashTransactions returns a hash of the transactions in the block
// 将交所有交易哈希然后建立默克尔树
func (b *Block) HashTransactions() []byte {
	var transactions [][]byte

	for _, tx := range b.Transactions {
		transactions = append(transactions, tx.Serialize())
	}
	//利用当前块的所有交易创建默克尔树得到默克尔根
	mTree := NewMerkleTree(transactions)

	return mTree.RootNode.Data
}

// Serialize serializes the block
func (b *Block) Serialize() []byte {
	var result bytes.Buffer
	encoder := gob.NewEncoder(&result)

	err := encoder.Encode(b)
	if err != nil {
		log.Panic(err)
	}

	return result.Bytes()
}

// DeserializeBlock deserializes a block
func DeserializeBlock(d []byte) *Block {
	var block Block

	decoder := gob.NewDecoder(bytes.NewReader(d))
	err := decoder.Decode(&block)
	if err != nil {
		log.Panic(err)
	}

	return &block
}
