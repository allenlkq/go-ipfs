package bstest

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	blocks "github.com/ipfs/go-ipfs/blocks"
	blockstore "github.com/ipfs/go-ipfs/blocks/blockstore"
	. "github.com/ipfs/go-ipfs/blockservice"
	offline "github.com/ipfs/go-ipfs/exchange/offline"

	cid "gx/ipfs/QmXUuRadqDq5BuFWzVU6VuKaSjTcNm1gNCtLvvP1TJCW4z/go-cid"
	u "gx/ipfs/Qmb912gdngC1UWwTkhuW8knyRbcWeu5kqkxBpveLmW8bSr/go-ipfs-util"
	ds "gx/ipfs/QmbzuUusHqaLLoNTDEVLcSF6vZDHZDLPC7p4bztRvvkXxU/go-datastore"
	dssync "gx/ipfs/QmbzuUusHqaLLoNTDEVLcSF6vZDHZDLPC7p4bztRvvkXxU/go-datastore/sync"
)

func newObject(data []byte) *testObject {
	return &testObject{
		Block: blocks.NewBlock(data),
	}
}

type testObject struct {
	blocks.Block
}

func (o *testObject) Cid() *cid.Cid {
	return cid.NewCidV0(o.Block.Multihash())
}

func TestBlocks(t *testing.T) {
	bstore := blockstore.NewBlockstore(dssync.MutexWrap(ds.NewMapDatastore()))
	bs := New(bstore, offline.Exchange(bstore))
	defer bs.Close()

	o := newObject([]byte("beep boop"))
	h := u.Hash([]byte("beep boop"))
	if !bytes.Equal(o.Multihash(), h) {
		t.Error("Block Multihash and data multihash not equal")
	}

	if !o.Cid().Equals(cid.NewCidV0(h)) {
		t.Error("Block key and data multihash key not equal")
	}

	k, err := bs.AddBlock(o)
	if err != nil {
		t.Error("failed to add block to BlockService", err)
		return
	}

	if !k.Equals(o.Cid()) {
		t.Error("returned key is not equal to block key", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	b2, err := bs.GetBlock(ctx, o.Cid())
	if err != nil {
		t.Error("failed to retrieve block from BlockService", err)
		return
	}

	if !o.Cid().Equals(b2.Cid()) {
		t.Error("Block keys not equal.")
	}

	if !bytes.Equal(o.RawData(), b2.RawData()) {
		t.Error("Block data is not equal.")
	}
}

func makeObjects(n int) []*testObject {
	var out []*testObject
	for i := 0; i < n; i++ {
		out = append(out, newObject([]byte(fmt.Sprintf("object %d", i))))
	}
	return out
}

func TestGetBlocksSequential(t *testing.T) {
	var servs = Mocks(4)
	for _, s := range servs {
		defer s.Close()
	}
	objs := makeObjects(50)

	var cids []*cid.Cid
	for _, o := range objs {
		cids = append(cids, o.Cid())
		servs[0].AddBlock(o)
	}

	t.Log("one instance at a time, get blocks concurrently")

	for i := 1; i < len(servs); i++ {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*50)
		defer cancel()
		out := servs[i].GetBlocks(ctx, cids)
		gotten := make(map[string]blocks.Block)
		for blk := range out {
			if _, ok := gotten[blk.Cid().KeyString()]; ok {
				t.Fatal("Got duplicate block!")
			}
			gotten[blk.Cid().KeyString()] = blk
		}
		if len(gotten) != len(objs) {
			t.Fatalf("Didnt get enough blocks back: %d/%d", len(gotten), len(objs))
		}
	}
}
