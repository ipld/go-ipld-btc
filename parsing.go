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
	r := bufio.NewReader(bytes.NewReader(b))
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
	out = append(out, txs...)

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
	return ReadBlock(bufio.NewReader(bytes.NewReader(b)))
}

func ReadBlock(r *bufio.Reader) (*Block, error) {
	var blk Block

	version, err := readFixedSlice(r, 4)
	if err != nil {
		return nil, fmt.Errorf("failed to read version: %s", err)
	}
	blk.Version = binary.LittleEndian.Uint32(version)

	prevBlock, err := readFixedSlice(r, 32)
	if err != nil {
		return nil, fmt.Errorf("failed to read prev_block: %s", err)
	}

	blkhash, _ := mh.Encode(prevBlock, mh.DBL_SHA2_256)
	blk.Parent = cid.NewCidV1(cid.BitcoinBlock, blkhash)

	merkleRoot, err := readFixedSlice(r, 32)
	if err != nil {
		return nil, fmt.Errorf("failed to read merkle_root: %s", err)
	}
	txroothash, _ := mh.Encode(merkleRoot, mh.DBL_SHA2_256)
	blk.MerkleRoot = cid.NewCidV1(cid.BitcoinTx, txroothash)

	timestamp, err := readFixedSlice(r, 4)
	if err != nil {
		return nil, fmt.Errorf("failed to read timestamp: %s", err)
	}
	blk.Timestamp = binary.LittleEndian.Uint32(timestamp)

	diff, err := readFixedSlice(r, 4)
	if err != nil {
		return nil, fmt.Errorf("failed to read difficulty: %s", err)
	}
	blk.Difficulty = binary.LittleEndian.Uint32(diff)

	nonce, err := readFixedSlice(r, 4)
	if err != nil {
		return nil, fmt.Errorf("failed to read nonce: %s", err)
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
	r := bufio.NewReader(bytes.NewReader(b))
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
func readTx(r *bufio.Reader) (*Tx, error) {
	rawVersion, err := readFixedSlice(r, 4)
	if err != nil {
		return nil, fmt.Errorf("failed to parse version: %s", err)
	}
	// version
	version := binary.LittleEndian.Uint32(rawVersion)

	isSegwit, err := isSegwitTx(r)
	if err != nil {
		return nil, fmt.Errorf("failed to check segwit: %s", err)
	}

	return readTxDetails(r, version, isSegwit)
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

func readTxDetails(r *bufio.Reader, version uint32, isSegwit bool) (*Tx, error) {
	if isSegwit {
		// header & flag validation already happened before
		_, err := r.Discard(2)
		if err != nil {
			return nil, err
		}
	}

	inputs, err := readTxInputs(r)
	if err != nil {
		return nil, err
	}

	outputs, err := readTxOutputs(r)
	if err != nil {
		return nil, err
	}

	var witnesses []*Witness
	if isSegwit {
		// witness
		// implicit witness_count == tx_in_count
		witnesses, err = readTxWitnesses(r, len(inputs))
		if err != nil {
			return nil, err
		}
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

func readTxWitnesses(r *bufio.Reader, ctr int) ([]*Witness, error) {
	witnesses := make([]*Witness, ctr)

	for i := 0; i < ctr; i++ {
		witCtr, err := readVarint(r)
		if err != nil {
			return nil, err
		}

		items := make([][]byte, witCtr)
		for j := 0; j < witCtr; j++ {
			item, err := readVarSlice(r)
			if err != nil {
				return nil, err
			}
			items[j] = item
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
	lockTime, err := readFixedSlice(r, 4)
	if err != nil {
		return 0, err
	}

	return binary.LittleEndian.Uint32(lockTime), nil
}

func parseTxIn(r *bufio.Reader) (*TxIn, error) {
	prevTxHash, err := readFixedSlice(r, 32)
	if err != nil {
		return nil, err
	}

	prevTxIndex, err := readFixedSlice(r, 4)
	if err != nil {
		return nil, err
	}

	script, err := readVarSlice(r)
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

	script, err := readVarSlice(r)
	if err != nil {
		return nil, err
	}

	// read script
	return &TxOut{
		Value:  binary.LittleEndian.Uint64(value),
		Script: script,
	}, nil
}

func readVarint(r *bufio.Reader) (int, error) {
	b, err := r.ReadByte()
	if err != nil {
		return 0, err
	}

	var res int
	switch b {
	case 0xfd:
		buf := make([]byte, 2)
		_, err := r.Read(buf)
		if err != nil {
			return 0, err
		}
		res = int(binary.LittleEndian.Uint16(buf))
	case 0xfe:
		buf := make([]byte, 4)
		_, err := r.Read(buf)
		if err != nil {
			return 0, err
		}

		res = int(binary.LittleEndian.Uint32(buf))
	case 0xff:
		buf := make([]byte, 8)
		_, err := r.Read(buf)
		if err != nil {
			return 0, err
		}

		res = int(binary.LittleEndian.Uint64(buf))
	default:
		res = int(b)
	}

	// all varints we are reading are actually unsigned,
	// so if they are suddenly signed this means there was an overflow
	// on int.
	if res < 0 {
		return 0, fmt.Errorf("varint overflow: %d", res)
	}

	return res, nil
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

func readVarSlice(r *bufio.Reader) ([]byte, error) {
	length, err := readVarint(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read length: %s", err)
	}

	return readFixedSlice(r, length)
}

func readFixedSlice(r *bufio.Reader, length int) ([]byte, error) {
	out := make([]byte, length)
	_, err := io.ReadFull(r, out)
	if err != nil {
		return nil, fmt.Errorf("failed to read all bytes(%d): %s", length, err)
	}

	return out, nil
}
