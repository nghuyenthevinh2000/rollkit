package node

import (
	"context"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/rollkit/rollkit/block"
	"github.com/rollkit/rollkit/types"
	"github.com/stretchr/testify/require"
)

// test running a full node that sends block data to bitcoin regtest
// the test setup will push blocks to da layer and proofs to bitcoin layer
// go test -count=1 -v -run ^TestBtcFetchSubmitProofs$ github.com/rollkit/rollkit/node
func TestBtcFetchSubmitProofs(t *testing.T) {
	// start a full node
	ctx1, cancel1 := context.WithCancel(context.Background())
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer func() {
		cancel1()
		cancel2()
	}()

	// randomize a number for 1 to 1000
	number := rand.Intn(1000) + 1
	chainId := types.TestChainID + string(rune(number))

	genesis, valPrivKey := types.GetGenesisWithPrivkey(chainId)

	// full aggregator node
	node1, signingKey := setupTestNode(ctx1, t, Full, true, chainId, genesis, valPrivKey)
	fullNode1, ok := node1.(*FullNode)
	require.True(t, ok)
	store1 := fullNode1.Store
	manager := fullNode1.blockManager
	height := store1.Height()

	// full query node
	node2, _ := setupTestNode(ctx2, t, Full, false, chainId, genesis, valPrivKey)
	fullNode2, ok := node2.(*FullNode)
	require.True(t, ok)
	manager2 := fullNode2.blockManager

	config := types.BlockConfig{
		Height:  height + 1,
		NTxs:    0,
		PrivKey: signingKey,
	}

	b1, _ := types.GenerateRandomBlockCustom(&config)

	// update state with hashes generated from block
	state, err := store1.GetState(ctx1)
	require.NoError(t, err)
	state.AppHash = b1.SignedHeader.AppHash
	state.LastResultsHash = b1.SignedHeader.LastResultsHash
	manager.SetLastState(state)
	manager2.SetLastState(state)

	// start full aggregate node
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		startNodeWithCleanup(t, node1)
	}()

	go func() {
		// sleep for 10 seconds to allow node 1 to start and submit blobs to DummyDA
		// due to DummyDA GetIDs() at exact height, we need to wait for node 1 to submit blobs before starting node 2
		// if still get error related to node2 not getting a block, increase sleep time
		time.Sleep(10 * time.Second)

		startNodeWithCleanup(t, node2)
	}()

	// check state of the first 20 blocks in both nodes
	go func() {
		height := uint64(1)
		for {
			time.Sleep(700 * time.Millisecond)
			block, err := manager2.GetBtcRollUpsBlockFromStore(ctx2, height)
			if err != nil {
				t.Logf("error getting block: %v", err)
				continue
			}

			t.Logf("block: %v", block.Height)
			height++

			if height == 20 {
				break
			}
		}

		wg.Done()
	}()

	wg.Wait()

	// check both stores from two nodes to ensure that they match
	for i := uint64(1); i < 20; i++ {
		b1, err := manager.GetBtcRollUpsBlockFromStore(ctx1, i)
		require.NoError(t, err)

		b2, err := manager2.GetBtcRollUpsBlockFromStore(ctx2, i)
		require.NoError(t, err)

		require.Equal(t, b1.Height, b2.Height)
		require.Equal(t, b1.BlockProofs, b2.BlockProofs)
		require.Equal(t, b1.TxOrderProofs, b2.TxOrderProofs)
	}

	// verify proofs from btc against roll ups blocks in manager 2
	// rollkit pushes blocks to DA in batch
	// the first batch pushed to DA has 15 blocks stored
	for i := uint64(1); i < 16; i++ {
		b1, err := manager2.GetBlockFromStore(ctx2, i)
		require.NoError(t, err)

		b2, err := manager2.GetBtcRollUpsBlockFromStore(ctx2, i)
		require.NoError(t, err)

		p, err := block.ConvertBlockToProofs(b1)
		require.NoError(t, err)

		require.Equal(t, p.Height, b2.Height)
		require.Equal(t, p.BlockProofs, b2.BlockProofs)
		require.Equal(t, p.TxOrderProofs, b2.TxOrderProofs)
	}
}
