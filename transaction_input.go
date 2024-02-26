package main

import "bytes"

// TXInput represents a transaction input
type TXInput struct {
	//表示引用的交易的哈希
	Txid []byte
	//表示引用交易的输出索引
	Vout int
	//表示交易输入的签名，验证输入的所有者有权花费引用的交易输出
	Signature []byte
	//公钥，在一个交易输入中用于验证签名
	PubKey []byte
}

// UsesKey checks whether the address initiated the transaction
func (in *TXInput) UsesKey(pubKeyHash []byte) bool {
	lockingHash := HashPubKey(in.PubKey)

	return bytes.Compare(lockingHash, pubKeyHash) == 0
}
