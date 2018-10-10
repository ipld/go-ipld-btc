package ipldbtc

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"

	cid "github.com/ipfs/go-cid"
	node "github.com/ipfs/go-ipld-format"
	mh "github.com/multiformats/go-multihash"
)

type Block struct {
	rawdata []byte

	Version    uint32  `json:"version"`
	Parent     cid.Cid `json:"parent"`
	MerkleRoot cid.Cid `json:"tx"`
	Timestamp  uint32  `json:"timestamp"`
	Difficulty uint32  `json:"difficulty"`
	Nonce      uint32  `json:"nonce"`

	cid cid.Cid
}

type Link struct {
	Target cid.Cid
}

// assert that Block matches the Node interface for ipld
var _ node.Node = (*Block)(nil)

func (b *Block) Cid() cid.Cid {
	h, _ := mh.Sum(b.header(), mh.DBL_SHA2_256, -1)
	return cid.NewCidV1(cid.BitcoinBlock, h)
}

func (b *Block) RawData() []byte {
	return b.header()
}

func (b *Block) Links() []*node.Link {
	return []*node.Link{
		{
			Name: "tx",
			Cid:  b.MerkleRoot,
		},
		{
			Name: "parent",
			Cid:  b.Parent,
		},
	}
}

func (b *Block) Loggable() map[string]interface{} {
	// TODO: more helpful info here
	return map[string]interface{}{
		"type": "bitcoin_block",
	}
}

// Resolve attempts to traverse a path through this block.
func (b *Block) Resolve(path []string) (interface{}, []string, error) {
	if len(path) == 0 {
		return nil, nil, fmt.Errorf("zero length path")
	}
	switch path[0] {
	case "version":
		return b.Version, path[1:], nil
	case "timestamp":
		return b.Timestamp, path[1:], nil
	case "difficulty":
		return b.Difficulty, path[1:], nil
	case "nonce":
		return b.Nonce, path[1:], nil
	case "parent":
		return &node.Link{Cid: b.Parent}, path[1:], nil
	case "tx":
		return &node.Link{Cid: b.MerkleRoot}, path[1:], nil
	default:
		return nil, nil, fmt.Errorf("no such link")
	}
}

// ResolveLink is a helper function that allows easier traversal of links through blocks
func (b *Block) ResolveLink(path []string) (*node.Link, []string, error) {
	out, rest, err := b.Resolve(path)
	if err != nil {
		return nil, nil, err
	}

	lnk, ok := out.(*node.Link)
	if !ok {
		return nil, nil, fmt.Errorf("object at path was not a link")
	}

	return lnk, rest, nil
}

func cidToHash(c cid.Cid) []byte {
	h := []byte(c.Hash())
	return h[len(h)-32:]
}

func hashToCid(hv []byte, t uint64) cid.Cid {
	h, _ := mh.Encode(hv, mh.DBL_SHA2_256)
	return cid.NewCidV1(t, h)
}

// header generates a serialized block header for this block
func (b *Block) header() []byte {
	buf := new(bytes.Buffer)

	i := make([]byte, 4)
	binary.LittleEndian.PutUint32(i, b.Version)
	buf.Write(i)

	buf.Write(cidToHash(b.Parent))
	buf.Write(cidToHash(b.MerkleRoot))

	binary.LittleEndian.PutUint32(i, b.Timestamp)
	buf.Write(i)

	binary.LittleEndian.PutUint32(i, b.Difficulty)
	buf.Write(i)

	binary.LittleEndian.PutUint32(i, b.Nonce)
	buf.Write(i)

	return buf.Bytes()
}

func (b *Block) Size() (uint64, error) {
	return uint64(len(b.rawdata)), nil
}

func (b *Block) Stat() (*node.NodeStat, error) {
	return &node.NodeStat{}, nil
}

func (b *Block) String() string {
	return fmt.Sprintf("[bitcoin block]")
}

func (b *Block) Tree(p string, depth int) []string {
	// TODO: this isnt a correct implementation yet
	return []string{"difficulty", "nonce", "version", "timestamp", "tx", "parent"}
}

func (b *Block) BTCSha() []byte {
	blkmh, _ := mh.Sum(b.header(), mh.DBL_SHA2_256, -1)
	return blkmh[2:]
}

func (b *Block) HexHash() string {
	return hex.EncodeToString(revString(b.BTCSha()))
}

func (b *Block) Copy() node.Node {
	nb := *b // cheating shallow copy
	return &nb
}

func revString(s []byte) []byte {
	b := make([]byte, len(s))
	for i, v := range []byte(s) {
		b[len(b)-(i+1)] = v
	}
	return b
}
