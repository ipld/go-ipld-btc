package ipldbtc

import (
	"bufio"
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
		return nil, fmt.Errorf("failed to read block header: %s", err)
	}

	if !bytes.Equal(blk.header(), b[:80]) {
		panic("not the same!")
	}

	nTx, err := readVarint(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read tx_count: %s", err)
	}
	fmt.Printf("txcount: %d\n", nTx)

	var txs []node.Node
	for i := 0; i < nTx; i++ {
		tx, err := readTx(r)
		if err != nil {
			return nil, fmt.Errorf("failed to read tx(%d): %s", i, err)
		}
		txs = append(txs, tx)
	}

	txtrees, err := mkMerkleTree(txs)
	if err != nil {
		return nil, fmt.Errorf("failed to mk merkle tree: %s", err)
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
		return nil, fmt.Errorf("failed to read version: %s", err)
	}
	blk.Version = binary.LittleEndian.Uint32(version)
	fmt.Printf("-- block version: %v %d\n", version, blk.Version)
	prevBlock := make([]byte, 32)
	_, err = io.ReadFull(r, prevBlock)
	if err != nil {
		return nil, fmt.Errorf("failed to read prev_block: %s", err)
	}

	blkhash, _ := mh.Encode(prevBlock, mh.DBL_SHA2_256)
	blk.Parent = cid.NewCidV1(cid.BitcoinBlock, blkhash)

	merkleRoot := make([]byte, 32)
	_, err = io.ReadFull(r, merkleRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to read merkle_root: %s", err)
	}
	txroothash, _ := mh.Encode(merkleRoot, mh.DBL_SHA2_256)
	blk.MerkleRoot = cid.NewCidV1(cid.BitcoinTx, txroothash)

	timestamp := make([]byte, 4)
	_, err = io.ReadFull(r, timestamp)
	if err != nil {
		return nil, fmt.Errorf("failed to read timestamp: %s", err)
	}
	blk.Timestamp = binary.LittleEndian.Uint32(timestamp)

	diff := make([]byte, 4)
	_, err = io.ReadFull(r, diff)
	if err != nil {
		return nil, fmt.Errorf("failed to read difficulty: %s", err)
	}
	blk.Difficulty = binary.LittleEndian.Uint32(diff)

	nonce := make([]byte, 4)
	_, err = io.ReadFull(r, nonce)
	if err != nil {
		return nil, fmt.Errorf("failed to read nonce: %s", err)
	}
	blk.Nonce = binary.LittleEndian.Uint32(nonce)
	fmt.Printf("nonce: %d\n", blk.Nonce)
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

// General layout of a transaction, before Segwit
//
// Bytes  | Name         | Data Type        | Description
// -------|--------------|------------------|------------
// 4      | version      | uint32_t         | Transaction version number;
// Varies | tx_in count  | compactSize uint | Number of inputs in this tx
// Varies | tx_in        | txIn             | Transaction inputs
// Varies | tx_out count | compactSize uint | Number of outputs in this tx
// Varies | tx_out       | txOut            | Transaction outputs.
// 4      | lock_time    | uint32_t         | A time (Unix epoch time) or block number
//
// With segwit the layout changes from
//
//   version | tx_in_count | tx_in | tx_out_count | tx_out | lock_time
//
//   version | marker | flag | tx_in_count | tx_in | tx_out_count | tx_out | witness | lock_time
//
func readTx(r *bytes.Reader) (*Tx, error) {
	rawVersion := make([]byte, 4)
	_, err := io.ReadFull(r, rawVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to parse version: %s", err)
	}
	// version
	version := binary.LittleEndian.Uint32(rawVersion)
	fmt.Printf("-- tx version: %d\n", version)
	buf := bufio.NewReader(r)

	isSegwit, err := isSegwitTx(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to check segwit: %s", err)
	}

	if isSegwit {
		return readSegwitTx(version, buf)
	}

	return readRegularTx(version, buf)
}

func isSegwitTx(r *bufio.Reader) (bool, error) {
	// the next two bytes must be [0x00, 0x01] to indicate the new
	// segwit format
	header, err := r.Peek(2)
	if err != nil {
		return false, err
	}

	if header[0] == 0x00 && header[1] == 0x01 {
		return true, nil
	}

	return false, nil
}

func readSegwitTx(version uint32, r *bufio.Reader) (*Tx, error) {
	next, _ := r.Peek(32)
	fmt.Printf("reading segwit: %v\n", next)

	// header & flag validation already happened before
	_, err := r.Discard(2)
	if err != nil {
		return nil, err
	}

	inputs, err := readTxInputs(r)
	if err != nil {
		return nil, err
	}

	outputs, err := readTxOutputs(r)
	if err != nil {
		return nil, err
	}

	// witness
	// implicit witness_count == tx_in_count
	witnesses, err := readTxWitnesses(r, len(inputs))
	if err != nil {
		return nil, err
	}

	lockTime, err := readTxLockTime(r)
	if err != nil {
		return nil, err
	}

	return &Tx{
		Version:   version,
		Inputs:    inputs,
		Outputs:   outputs,
		LockTime:  lockTime,
		Witnesses: witnesses,
	}, nil
}

func readRegularTx(version uint32, r *bufio.Reader) (*Tx, error) {
	fmt.Println("reading regular tx")
	inputs, err := readTxInputs(r)
	if err != nil {
		return nil, err
	}

	outputs, err := readTxOutputs(r)
	if err != nil {
		return nil, err
	}

	lockTime, err := readTxLockTime(r)
	if err != nil {
		return nil, err
	}

	return &Tx{
		Version:  version,
		Inputs:   inputs,
		Outputs:  outputs,
		LockTime: lockTime,
	}, nil
}

func readTxWitnesses(r *bufio.Reader, ctr int) ([]*Witness, error) {
	witnesses := make([]*Witness, ctr)
	fmt.Printf("readTxWitnesses: %d\n", ctr)

	for i := 0; i < ctr; i++ {
		witCtr, err := readVarint(r)
		if err != nil {
			return nil, err
		}
		fmt.Printf("witness field, %d\n", witCtr)
		items := make([][]byte, witCtr)
		for j := 0; j < witCtr; j++ {
			len, err := readVarint(r)
			if err != nil {
				return nil, err
			}
			if len == 0 {
				continue
			}

			items[j] = make([]byte, len)
			if _, err = io.ReadFull(r, items[j]); err != nil {
				return nil, err
			}

			fmt.Printf("item: %d %v\n", len, items[j])
		}

		witnesses[i] = &Witness{
			Data: items,
		}
	}

	return witnesses, nil
}

func readTxInputs(r *bufio.Reader) ([]*TxIn, error) {
	inCtr, err := readVarint(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read in_count: %s", err)
	}

	fmt.Printf("readtxinputs: %d\n", inCtr)
	out := make([]*TxIn, inCtr)

	for i := 0; i < inCtr; i++ {
		txin, err := parseTxIn(r)
		if err != nil {
			return nil, fmt.Errorf("failed to parse tx(%d): %s", i, err)
		}
		out[i] = txin
	}

	return out, nil
}

func readTxOutputs(r *bufio.Reader) ([]*TxOut, error) {
	outCtr, err := readVarint(r)
	if err != nil {
		return nil, err
	}

	out := make([]*TxOut, outCtr)

	for i := 0; i < outCtr; i++ {
		txout, err := parseTxOut(r)
		if err != nil {
			return nil, fmt.Errorf("failed to read tx_out(%d/%d): %s", i, outCtr, err)
		}

		out[i] = txout
	}

	return out, nil
}

func readTxLockTime(r *bufio.Reader) (uint32, error) {
	lockTime := make([]byte, 4)
	_, err := io.ReadFull(r, lockTime)
	if err != nil {
		return 0, err
	}
	fmt.Printf("locktime: %v\n", lockTime)
	return binary.LittleEndian.Uint32(lockTime), nil
}

func parseTxIn(r *bufio.Reader) (*TxIn, error) {
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

func parseTxOut(r *bufio.Reader) (*TxOut, error) {
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
		return nil, fmt.Errorf("not enough bytes (%d): %s", scriptLen, err)
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

func readVarint(r io.Reader) (int, error) {
	b := make([]byte, 1)
	_, err := r.Read(b)
	if err != nil {
		return 0, err
	}
	// fmt.Printf("readvarint: %d\n", b[0])
	switch b[0] {
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
		return int(b[0]), nil
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
