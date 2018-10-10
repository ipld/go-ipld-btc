package ipldbtc

import (
	"encoding/json"
	"fmt"

	cid "github.com/ipfs/go-cid"
	node "github.com/ipfs/go-ipld-format"
	mh "github.com/multiformats/go-multihash"
)

type TxTree struct {
	Left  *node.Link
	Right *node.Link
}

func (t *TxTree) BTCSha() []byte {
	return cidToHash(t.Cid())
}

func (t *TxTree) Cid() cid.Cid {
	h, _ := mh.Sum(t.RawData(), mh.DBL_SHA2_256, -1)
	return cid.NewCidV1(cid.BitcoinTx, h)
}

func (t *TxTree) Links() []*node.Link {
	return []*node.Link{t.Left, t.Right}
}

func (t *TxTree) RawData() []byte {
	out := make([]byte, 64)
	lbytes := cidToHash(t.Left.Cid)
	copy(out[:32], lbytes)

	rbytes := cidToHash(t.Right.Cid)
	copy(out[32:], rbytes)

	return out
}

func (t *TxTree) Loggable() map[string]interface{} {
	return map[string]interface{}{
		"type": "bitcoin_tx_tree",
	}
}

func (t *TxTree) Resolve(path []string) (interface{}, []string, error) {
	if len(path) == 0 {
		return nil, nil, fmt.Errorf("zero length path")
	}

	switch path[0] {
	case "0":
		return t.Left, path[1:], nil
	case "1":
		return t.Right, path[1:], nil
	default:
		return nil, nil, fmt.Errorf("no such link")
	}
}

func (t *TxTree) MarshalJSON() ([]byte, error) {
	return json.Marshal([]*Link{{t.Left.Cid}, {t.Right.Cid}})
}

func (t *TxTree) Copy() node.Node {
	nt := *t
	return &nt
}

func (t *TxTree) ResolveLink(path []string) (*node.Link, []string, error) {
	out, rest, err := t.Resolve(path)
	if err != nil {
		return nil, nil, err
	}

	lnk, ok := out.(*node.Link)
	if ok {
		return lnk, rest, nil
	}

	return nil, nil, fmt.Errorf("path did not lead to link")
}

func (t *TxTree) Size() (uint64, error) {
	return uint64(len(t.RawData())), nil
}

func (t *TxTree) Stat() (*node.NodeStat, error) {
	return &node.NodeStat{}, nil
}

func (t *TxTree) String() string {
	return fmt.Sprintf("[bitcoin transaction tree]")
}

func (t *TxTree) Tree(p string, depth int) []string {
	return []string{"0", "1"}
}
