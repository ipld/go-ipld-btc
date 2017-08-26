package ipldbtc

import (
	"encoding/hex"
	"io/ioutil"
	"testing"
)

var txdata = "01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff6403a6ab05e4b883e5bda9e7a59ee4bb99e9b1bc76a3a2bb0e9c92f06e4a6349de9ccc8fbe0fad11133ed73c78ee12876334c13c02000000f09f909f2f4249503130302f4d696e65642062792073647a6861626364000000000000000000000000000000005f77dba4015ca34297000000001976a914c825a1ecf2a6830c4401620c3a16f1995057c2ab88acfe75853a"

func TestBlockMessageDecoding(t *testing.T) {
	hexdata, err := ioutil.ReadFile("block.bin")
	if err != nil {
		t.Fatal(err)
	}

	data, err := hex.DecodeString(string(hexdata[:len(hexdata)-1]))
	if err != nil {
		t.Fatal(err)
	}

	nodes, err := DecodeBlockMessage(data)
	if err != nil {
		t.Fatal(err)
	}

	expblk := "0000000000000002909eabb1da3710351faf452374946a0dfdb247d491c6c23e"
	if nodes[0].(*Block).HexHash() != expblk {
		t.Fatal("parsed incorrectly")
	}

	blk, _, err := nodes[0].ResolveLink([]string{"tx"})
	if err != nil {
		t.Fatal(err)
	}

	if !blk.Cid.Equals(nodes[len(nodes)-1].Cid()) {
		t.Fatal("merkle root looks wrong")
	}
}

func TestBlockMessageDecodingSegwit(t *testing.T) {
	hexdata, err := ioutil.ReadFile("segwit.bin")
	if err != nil {
		t.Fatal(err)
	}

	data, err := hex.DecodeString(string(hexdata[:len(hexdata)-1]))
	if err != nil {
		t.Fatal(err)
	}

	nodes, err := DecodeBlockMessage(data)
	if err != nil {
		t.Fatal(err)
	}

	expblk := "0000000000000002909eabb1da3710351faf452374946a0dfdb247d491c6c23e"
	if nodes[0].(*Block).HexHash() != expblk {
		t.Fatal("parsed incorrectly")
	}

	blk, _, err := nodes[0].ResolveLink([]string{"tx"})
	if err != nil {
		t.Fatal(err)
	}

	if !blk.Cid.Equals(nodes[len(nodes)-1].Cid()) {
		t.Fatal("merkle root looks wrong")
	}
}

func TestDecodingNoTxs(t *testing.T) {
	hexdata := "010000000508085c47cc849eb80ea905cc7800a3be674ffc57263cf210c59d8d00000000112ba175a1e04b14ba9e7ea5f76ab640affeef5ec98173ac9799a852fa39add320cd6649ffff001d1e2de5650101000000010000000000000000000000000000000000000000000000000000000000000000ffffffff0704ffff001d0136ffffffff0100f2052a01000000434104fcc2888ca91cf0103d8c5797c256bf976e81f280205d002d85b9b622ed1a6f820866c7b5fe12285cfa78c035355d752fc94a398b67597dc4fbb5b386816425ddac00000000"

	data, err := hex.DecodeString(hexdata)
	if err != nil {
		t.Fatal(err)
	}

	nodes, err := DecodeBlockMessage(data)
	if err != nil {
		t.Fatal(err)
	}

	expblk := "000000002c05cc2e78923c34df87fd108b22221ac6076c18f3ade378a4d915e9"

	blk := nodes[0].(*Block)
	if nodes[0].(*Block).HexHash() != expblk {
		t.Fatal("parsed incorrectly")
	}

	tx := nodes[1].(*Tx)
	t.Log(blk.MerkleRoot)
	t.Log(tx.Cid())

}

func TestTxDecoding(t *testing.T) {
	data, err := hex.DecodeString(txdata)
	if err != nil {
		t.Fatal(err)
	}

	tx, err := DecodeTx(data)
	if err != nil {
		t.Fatal(err)
	}

	if tx.LockTime != 981825022 {
		t.Fatal("lock time incorrect")
	}

	if tx.Inputs[0].SeqNo != 2765846367 {
		t.Fatal("seqno not right")
	}
}
