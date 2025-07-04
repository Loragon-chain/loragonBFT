// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package comm

import (
	"context"
	"fmt"
	"time"

	"github.com/Loragon-chain/loragonBFT/block"
	"github.com/Loragon-chain/loragonBFT/libs/co"
	"github.com/Loragon-chain/loragonBFT/libs/comm/proto"
	"github.com/Loragon-chain/loragonBFT/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/pkg/errors"
)

func (c *Communicator) sync(peer *Peer, headNum uint32, handler HandleBlockStream) error {
	ancestor, err := c.findCommonAncestor(peer, headNum)
	if err != nil {
		return errors.WithMessage(err, "find common ancestor")
	}
	fromNum := ancestor + 1

	// it's important to set cap to 2
	errCh := make(chan error, 2)

	ctx, cancel := context.WithCancel(c.ctx)
	blockCh := make(chan *block.EscortedBlock, 4096)

	var goes co.Goes
	goes.Go(func() {
		defer cancel()
		if err := handler(ctx, blockCh); err != nil {
			errCh <- err
		}
	})
	goes.Go(func() {
		defer close(blockCh)
		var blocks []*block.EscortedBlock
		for {
			start := time.Now()
			result, err := proto.GetBlocksFromNumber(ctx, peer, fromNum)
			if err != nil {
				errCh <- err
				return
			}
			if len(result) > 0 {
				c.logger.Info(fmt.Sprintf("downloaded blocks(%d) from %d", len(result), fromNum), "peer", peer.String(), "elapsed", types.PrettyDuration(time.Since(start)))
			}
			if len(result) == 0 {
				return
			}

			blocks = blocks[:0]
			for _, raw := range result {
				var blk block.EscortedBlock
				if err := rlp.DecodeBytes(raw, &blk); err != nil {
					errCh <- errors.Wrap(err, "invalid block")
					return
				}
				if blk.Block.Number() != fromNum {
					errCh <- errors.New("broken sequence")
					return
				}
				fromNum++
				blocks = append(blocks, &blk)

			}

			<-co.Parallel(func(queue chan<- func()) {
				for _, blk := range blocks {
					h := blk.Block.Header()
					queue <- func() { h.ID() }
					for _, tx := range blk.Block.Transactions() {
						tx := tx
						queue <- func() {
							tx.Hash()
						}
					}
				}
			})

			for _, blk := range blocks {
				peer.MarkBlock(blk.Block.ID())
				select {
				case <-ctx.Done():
					return
				case blockCh <- blk:
					// log.Info("Put in block chan", "blk", blk.Block.Number(), "len", len(blockCh), "cap", cap(blockCh))
				}
			}
		}
	})
	goes.Wait()

	select {
	case err := <-errCh:
		return err
	default:
		return nil
	}
}

func (c *Communicator) findCommonAncestor(peer *Peer, headNum uint32) (uint32, error) {
	if headNum == 0 {
		return headNum, nil
	}

	isOverlapped := func(num uint32) (bool, error) {
		result, err := proto.GetBlockIDByNumber(c.ctx, peer, num)
		if err != nil {
			return false, err
		}
		id, err := c.chain.GetTrunkBlockID(num)
		if err != nil {
			return false, err
		}
		return id == result, nil
	}

	var find func(start uint32, end uint32, ancestor uint32) (uint32, error)
	find = func(start uint32, end uint32, ancestor uint32) (uint32, error) {
		if start == end {
			overlapped, err := isOverlapped(start)
			if err != nil {
				return 0, err
			}
			if overlapped {
				return start, nil
			}
		} else {
			mid := (start + end) / 2
			overlapped, err := isOverlapped(mid)
			if err != nil {
				return 0, err
			}
			if overlapped {
				return find(mid+1, end, mid)
			}

			if mid > start {
				return find(start, mid-1, ancestor)
			}
		}
		return ancestor, nil
	}

	fastSeek := func() (uint32, error) {
		var backward uint32
		for {
			if backward >= headNum {
				return 0, nil
			}

			overlapped, err := isOverlapped(headNum - backward)
			if err != nil {
				return 0, err
			}
			if overlapped {
				return headNum - backward, nil
			}
			if backward == 0 {
				backward = 1
			} else {
				backward <<= 1
			}
		}
	}

	seekNum, err := fastSeek()
	if err != nil {
		return 0, err
	}
	if seekNum == headNum {
		return headNum, nil
	}
	return find(seekNum, headNum, 0)
}

func (c *Communicator) syncTxs(peer *Peer) {
	for i := 0; ; i++ {
		peer.logger.Debug(fmt.Sprintf("sync txs loop %v", i))
		result, err := proto.GetTxs(c.ctx, peer)
		if err != nil {
			peer.logger.Debug("failed to request txs", "err", err)
			return
		}

		// no more txs
		if len(result) == 0 {
			break
		}

		for _, tx := range result {
			peer.MarkTransaction(tx.Hash())
			c.txPool.StrictlyAdd(tx)
			select {
			case <-c.ctx.Done():
				return
			default:
			}
		}

		if i >= 100 {
			peer.logger.Debug("too many loops to sync txs, break")
			return
		}
	}
	peer.logger.Debug("sync txs done")
}
