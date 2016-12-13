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
	_ = expblk

	blk, _, err := nodes[0].ResolveLink([]string{"tx"})
	if err != nil {
		t.Fatal(err)
	}

	if !blk.Cid.Equals(nodes[len(nodes)-1].Cid()) {
		t.Fatal("merkle root looks wrong")
	}
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
