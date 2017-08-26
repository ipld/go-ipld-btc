package ipldbtc

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"fmt"
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

	data := make([]byte, len(hexdata)-1)
	_, err = hex.Decode(data, hexdata[:len(hexdata)-1])
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("data: %v\n", data[0:10])
	nodes, err := DecodeBlockMessage(data)
	if err != nil {
		t.Fatal(err)
	}

	block := nodes[0].(*Block)

	if block.Version != 536870914 {
		t.Fatalf("incorrect version: %d", block.Version)
	}

	if block.Nonce != 3781004001 {
		t.Fatalf("incorrect nonce: %d", block.Nonce)
	}

	expblk := "00000000000000000006d921ce47d509544dec06838a2ff9303c50d12f4a0199"
	if block.HexHash() != expblk {
		t.Fatalf("parsed incorrectly: %s", block.HexHash())
	}

	blk, _, err := nodes[0].ResolveLink([]string{"tx"})
	if err != nil {
		t.Fatal(err)
	}

	if !blk.Cid.Equals(nodes[len(nodes)-1].Cid()) {
		t.Fatal("merkle root looks wrong")
	}
}

func TestBip143TxsNativeP2WPKH(t *testing.T) {
	hexData := "01000000000102fff7f7881a8099afa6940d42d1e7f6362bec38171ea3edf433541db4e4ad969f00000000494830450221008b9d1dc26ba6a9cb62127b02742fa9d754cd3bebf337f7a55d114c8e5cdd30be022040529b194ba3f9281a99f2b1c0a19c0489bc22ede944ccf4ecbab4cc618ef3ed01eeffffffef51e1b804cc89d182d279655c3aa89e815b1b309fe287d9b2b55d57b90ec68a0100000000ffffffff02202cb206000000001976a9148280b37df378db99f66f85c95a783a76ac7a6d5988ac9093510d000000001976a9143bde42dbee7e4dbe6a21b2d50ce2f0167faa815988ac000247304402203609e17b84f6a7d30c80bfa610b5b4542f32a8a0d5447a12fb1366d7f01cc44a0220573a954c4518331561406f90300e8f3358f51928d43c212a8caed02de67eebee0121025476c2e83188368da1ff3e292e7acafcdb3566bb0ad253f62fc70f07aeee635711000000"

	data, err := hex.DecodeString(hexData)
	if err != nil {
		t.Fatal(err)
	}

	tx, err := readTx(bufio.NewReader(bytes.NewReader(data)))
	if err != nil {
		t.Fatal(err)
	}

	if tx.Version != 1 {
		t.Fatal("incorrect version")
	}

	if len(tx.Inputs) != 2 {
		t.Fatal("incorrect input length")
	}

	if len(tx.Outputs) != 2 {
		t.Fatal("incorrect output length")
	}
	if len(tx.Witnesses) != 2 {
		t.Fatal("incorrect witnesses length")
	}

	if len(tx.Witnesses[0].Data) > 0 {
		t.Fatal("incorrect first witness")
	}

	witAHex := "304402203609e17b84f6a7d30c80bfa610b5b4542f32a8a0d5447a12fb1366d7f01cc44a0220573a954c4518331561406f90300e8f3358f51928d43c212a8caed02de67eebee01"
	witBHex := "025476c2e83188368da1ff3e292e7acafcdb3566bb0ad253f62fc70f07aeee6357"
	witA, err := hex.DecodeString(witAHex)
	if err != nil {
		t.Fatal(err)
	}
	witB, err := hex.DecodeString(witBHex)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(tx.Witnesses[1].Data[0], witA) {
		t.Fatal("incorrect second witness 1")
	}

	if !bytes.Equal(tx.Witnesses[1].Data[1], witB) {
		t.Fatal("incorrect second witness 2")
	}

	if tx.LockTime != 17 /* 0x11000000 */ {
		t.Fatalf("incorrect lock time: %d", tx.LockTime)
	}
}

func TestBip143TxsNativeP2WSH(t *testing.T) {
	hexData := "01000000000102e9b542c5176808107ff1df906f46bb1f2583b16112b95ee5380665ba7fcfc0010000000000ffffffff80e68831516392fcd100d186b3c2c7b95c80b53c77e77c35ba03a66b429a2a1b0000000000ffffffff0280969800000000001976a914de4b231626ef508c9a74a8517e6783c0546d6b2888ac80969800000000001976a9146648a8cd4531e1ec47f35916de8e259237294d1e88ac02483045022100f6a10b8604e6dc910194b79ccfc93e1bc0ec7c03453caaa8987f7d6c3413566002206216229ede9b4d6ec2d325be245c5b508ff0339bf1794078e20bfe0babc7ffe683270063ab68210392972e2eb617b2388771abe27235fd5ac44af8e61693261550447a4c3e39da98ac024730440220032521802a76ad7bf74d0e2c218b72cf0cbc867066e2e53db905ba37f130397e02207709e2188ed7f08f4c952d9d13986da504502b8c3be59617e043552f506c46ff83275163ab68210392972e2eb617b2388771abe27235fd5ac44af8e61693261550447a4c3e39da98ac00000000"

	data, err := hex.DecodeString(hexData)
	if err != nil {
		t.Fatal(err)
	}

	tx, err := readTx(bufio.NewReader(bytes.NewReader(data)))
	if err != nil {
		t.Fatal(err)
	}

	if tx.Version != 1 {
		t.Fatal("incorrect version")
	}

	if len(tx.Inputs) != 2 {
		t.Fatal("incorrect input length")
	}

	if len(tx.Outputs) != 2 {
		t.Fatal("incorrect output length")
	}
	if len(tx.Witnesses) != 2 {
		t.Fatal("incorrect witnesses length")
	}

	witA1Hex := "3045022100f6a10b8604e6dc910194b79ccfc93e1bc0ec7c03453caaa8987f7d6c3413566002206216229ede9b4d6ec2d325be245c5b508ff0339bf1794078e20bfe0babc7ffe683"

	witB1Hex := "0063ab68210392972e2eb617b2388771abe27235fd5ac44af8e61693261550447a4c3e39da98ac"

	witA1, err := hex.DecodeString(witA1Hex)
	if err != nil {
		t.Fatal(err)
	}
	witB1, err := hex.DecodeString(witB1Hex)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(tx.Witnesses[0].Data[0], witA1) {
		t.Fatal("incorrect first witness 1")
	}

	if !bytes.Equal(tx.Witnesses[0].Data[1], witB1) {
		t.Fatal("incorrect first witness 2")
	}

	witA2Hex := "30440220032521802a76ad7bf74d0e2c218b72cf0cbc867066e2e53db905ba37f130397e02207709e2188ed7f08f4c952d9d13986da504502b8c3be59617e043552f506c46ff83"
	witB2Hex := "5163ab68210392972e2eb617b2388771abe27235fd5ac44af8e61693261550447a4c3e39da98ac"
	witA2, err := hex.DecodeString(witA2Hex)
	if err != nil {
		t.Fatal(err)
	}
	witB2, err := hex.DecodeString(witB2Hex)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(tx.Witnesses[1].Data[0], witA2) {
		t.Fatal("incorrect second witness 1")
	}

	if !bytes.Equal(tx.Witnesses[1].Data[1], witB2) {
		t.Fatal("incorrect second witness 2")
	}

	if tx.LockTime != 0 /* 0x00000000 */ {
		t.Fatalf("incorrect lock time: %d", tx.LockTime)
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
