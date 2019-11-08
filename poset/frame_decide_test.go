package poset

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/assert"

	"github.com/Fantom-foundation/go-lachesis/inter"
	"github.com/Fantom-foundation/go-lachesis/inter/idx"
	"github.com/Fantom-foundation/go-lachesis/logger"
)

func TestConfirmBlockEvents(t *testing.T) {
	logger.SetTestMode(t)
	assertar := assert.New(t)

	nodes := inter.GenNodes(5)
	poset, _, input := FakePoset("", nodes)

	var (
		frames []idx.Frame
		blocks []*inter.Block
	)
	applyBlock := poset.callback.ApplyBlock
	poset.callback.ApplyBlock = func(arg inter.ApplyBlockArgs) (common.Hash, bool) {
		frames = append(frames, poset.LastDecidedFrame)
		blocks = append(blocks, arg.Block)

		return applyBlock(arg)
	}

	eventCount := int(poset.dag.EpochLen)
	_ = inter.ForEachRandEvent(nodes, eventCount, poset.dag.MaxParents, nil, inter.ForEachEvent{
		Process: func(e *inter.Event, name string) {
			input.SetEvent(e)
			assertar.NoError(
				poset.ProcessEvent(e))
			assertar.NoError(
				flushDb(poset, e.Hash()))

		},
		Build: func(e *inter.Event, name string) *inter.Event {
			e.Epoch = idx.Epoch(1)
			if e.Seq%2 != 0 {
				e.Transactions = append(e.Transactions, &types.Transaction{})
			}
			e.TxHash = types.DeriveSha(e.Transactions)
			return poset.Prepare(e)
		},
	})

	// unconfirm all events
	it := poset.store.table.ConfirmedEvent.NewIterator()
	batch := poset.store.table.ConfirmedEvent.NewBatch()
	for it.Next() {
		assertar.NoError(batch.Delete(it.Key()))
	}
	assertar.NoError(batch.Write())
	it.Release()

	for i, block := range blocks {
		frame := frames[i]
		atropos := blocks[i].Atropos

		// call confirmBlock again
		gotBlock, cheaters, _ := poset.confirmBlock(frame, atropos)

		if !assertar.Empty(cheaters) {
			break
		}
		if !assertar.Equal(block.Events, gotBlock.Events) {
			break
		}
	}
}
