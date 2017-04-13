package ipldbtc

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	cid "github.com/ipfs/go-cid"
	node "github.com/ipfs/go-ipld-format"
	mh "github.com/multiformats/go-multihash"
)

func DecodeBlockMessage(b []byte) ([]node.Node, error) {
	r := bytes.NewReader(b)
	blk, err := ReadBlock(r)
	if err != nil {
		return nil, err
	}

	if !bytes.Equal(blk.header(), b[:80]) {
		panic("not the same!")
	}

	nTx, err := readVarint(r)
	if err != nil {
		return nil, err
	}

	var txs []node.Node
	for i := 0; i < nTx; i++ {
		tx, err := readTx(r)
		if err != nil {
			return nil, err
		}

		txs = append(txs, tx)
	}

	txtrees, err := mkMerkleTree(txs)
	if err != nil {
		return nil, err
	}

	out := []node.Node{blk}
	for _, tx := range txs {
		out = append(out, tx)
	}

	for _, txtree := range txtrees {
		out = append(out, txtree)
	}

	return out, nil
}

func mkMerkleTree(txs []node.Node) ([]*TxTree, error) {
	var out []*TxTree
	var next []node.Node
	layer := txs
	for len(layer) > 1 {
		if len(layer)%2 != 0 {
			layer = append(layer, layer[len(layer)-1])
		}
		for i := 0; i < len(layer)/2; i++ {
			var left, right node.Node
			left = layer[i*2]
			right = layer[(i*2)+1]

			t := &TxTree{
				Left:  &node.Link{Cid: left.Cid()},
				Right: &node.Link{Cid: right.Cid()},
			}

			out = append(out, t)
			next = append(next, t)
		}

		layer = next
		next = nil
	}

	return out, nil
}

func DecodeBlock(b []byte) (*Block, error) {
	return ReadBlock(bytes.NewReader(b))
}

func ReadBlock(r *bytes.Reader) (*Block, error) {
	var blk Block

	version := make([]byte, 4)
	_, err := io.ReadFull(r, version)
	if err != nil {
		return nil, err
	}
	blk.Version = binary.LittleEndian.Uint32(version)

	prevBlock := make([]byte, 32)
	_, err = io.ReadFull(r, prevBlock)
	if err != nil {
		return nil, err
	}

	blkhash, _ := mh.Encode(prevBlock, mh.DBL_SHA2_256)
	blk.Parent = cid.NewCidV1(cid.BitcoinBlock, blkhash)

	merkleRoot := make([]byte, 32)
	_, err = io.ReadFull(r, merkleRoot)
	if err != nil {
		return nil, err
	}
	txroothash, _ := mh.Encode(merkleRoot, mh.DBL_SHA2_256)
	blk.MerkleRoot = cid.NewCidV1(cid.BitcoinTx, txroothash)

	timestamp := make([]byte, 4)
	_, err = io.ReadFull(r, timestamp)
	if err != nil {
		return nil, err
	}
	blk.Timestamp = binary.LittleEndian.Uint32(timestamp)

	diff := make([]byte, 4)
	_, err = io.ReadFull(r, diff)
	if err != nil {
		return nil, err
	}
	blk.Difficulty = binary.LittleEndian.Uint32(diff)

	nonce := make([]byte, 4)
	_, err = io.ReadFull(r, nonce)
	if err != nil {
		return nil, err
	}
	blk.Nonce = binary.LittleEndian.Uint32(nonce)

	return &blk, nil
}

func DecodeMaybeTx(b []byte) (node.Node, error) {
	if len(b) == 64 {
		return DecodeTxTree(b)
	}
	return DecodeTx(b)
}

func DecodeTx(b []byte) (*Tx, error) {
	r := bytes.NewReader(b)
	return readTx(r)
}

func DecodeTxTree(b []byte) (*TxTree, error) {
	if len(b) != 64 {
		return nil, fmt.Errorf("invalid tx tree data")
	}

	lnkL := txHashToLink(b[:32])
	lnkR := txHashToLink(b[32:])
	return &TxTree{
		Left:  lnkL,
		Right: lnkR,
	}, nil
}

func readTx(r *bytes.Reader) (*Tx, error) {
	var out Tx

	version := make([]byte, 4)
	_, err := io.ReadFull(r, version)
	if err != nil {
		return nil, err
	}
	out.Version = binary.LittleEndian.Uint32(version)

	inCtr, err := readVarint(r)
	if err != nil {
		return nil, err
	}

	for i := 0; i < inCtr; i++ {
		txin, err := parseTxIn(r)
		if err != nil {
			return nil, err
		}

		out.Inputs = append(out.Inputs, txin)
	}

	outCtr, err := readVarint(r)
	if err != nil {
		return nil, err
	}

	for i := 0; i < outCtr; i++ {
		txout, err := parseTxOut(r)
		if err != nil {
			return nil, err
		}

		out.Outputs = append(out.Outputs, txout)
	}

	lock_time := make([]byte, 4)
	_, err = io.ReadFull(r, lock_time)
	if err != nil {
		return nil, err
	}

	out.LockTime = binary.LittleEndian.Uint32(lock_time)

	return &out, nil
}

func parseTxIn(r *bytes.Reader) (*TxIn, error) {
	prevTxHash := make([]byte, 32)
	_, err := io.ReadFull(r, prevTxHash)
	if err != nil {
		return nil, err
	}

	prevTxIndex := make([]byte, 4)
	_, err = io.ReadFull(r, prevTxIndex)
	if err != nil {
		return nil, err
	}

	scriptLen, err := readVarint(r)
	if err != nil {
		return nil, err
	}

	// Read Script
	script := make([]byte, scriptLen)
	_, err = io.ReadFull(r, script)
	if err != nil {
		return nil, err
	}

	seqNo := make([]byte, 4)
	_, err = io.ReadFull(r, seqNo)
	if err != nil {
		return nil, err
	}

	return &TxIn{
		PrevTx:      hashToCid(prevTxHash, cid.BitcoinTx),
		PrevTxIndex: binary.LittleEndian.Uint32(prevTxIndex),
		Script:      script,
		SeqNo:       binary.LittleEndian.Uint32(seqNo),
	}, nil
}

func parseTxOut(r *bytes.Reader) (*TxOut, error) {
	value := make([]byte, 8)
	_, err := io.ReadFull(r, value)
	if err != nil {
		return nil, err
	}

	scriptLen, err := readVarint(r)
	if err != nil {
		return nil, err
	}

	script := make([]byte, scriptLen)
	_, err = io.ReadFull(r, script)
	if err != nil {
		return nil, err
	}

	// read script
	return &TxOut{
		Value:  binary.LittleEndian.Uint64(value),
		Script: script,
	}, nil
}

func readBuf(r io.Reader, size int) ([]byte, error) {
	out := make([]byte, size)
	_, err := io.ReadFull(r, out)
	return out, err
}

func readVarint(r *bytes.Reader) (int, error) {
	b, err := r.ReadByte()
	if err != nil {
		return 0, err
	}

	switch b {
	case 0xfd:
		buf := make([]byte, 2)
		_, err := r.Read(buf)
		if err != nil {
			return 0, err
		}
		return int(binary.LittleEndian.Uint16(buf)), nil
	case 0xfe:
		buf := make([]byte, 4)
		_, err := r.Read(buf)
		if err != nil {
			return 0, err
		}

		return int(binary.LittleEndian.Uint32(buf)), nil
	case 0xff:
		buf := make([]byte, 8)
		_, err := r.Read(buf)
		if err != nil {
			return 0, err
		}

		return int(binary.LittleEndian.Uint64(buf)), nil
	default:
		return int(b), nil
	}
}

func writeVarInt(w io.Writer, n uint64) error {
	var d []byte
	if n < 0xFD {
		d = []byte{byte(n)}
	} else if n <= 0xFFFF {
		d = make([]byte, 3)
		binary.LittleEndian.PutUint16(d[1:], uint16(n))
		d[0] = 0xFD
	} else if n <= 0xFFFFFFF {
		d = make([]byte, 5)
		binary.LittleEndian.PutUint32(d[1:], uint32(n))
		d[0] = 0xFE
	} else {
		d = make([]byte, 9)
		binary.LittleEndian.PutUint64(d[1:], n)
		d[0] = 0xFE
	}
	_, err := w.Write(d)
	return err
}
