// Copyright (c) 2020 The Meter.io developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package chain

import (
	"sync/atomic"

	"github.com/Loragon-chain/loragonBFT/block"
)

type rawBlock struct {
	raw    block.Raw
	header atomic.Value
	body   atomic.Value
	block  atomic.Value
}

func newRawBlock(raw block.Raw, block *block.Block) *rawBlock {
	rb := &rawBlock{raw: raw}
	rb.header.Store(block.Header())
	rb.body.Store(block.Body())
	rb.block.Store(block)
	return rb
}

func (rb *rawBlock) Header() (*block.Header, error) {
	if cached := rb.header.Load(); cached != nil {
		return cached.(*block.Header), nil
	}

	h, err := rb.raw.DecodeHeader()
	if err != nil {
		return nil, err
	}
	rb.header.Store(h)
	return h, nil
}

func (rb *rawBlock) Body() (*block.Body, error) {
	if cached := rb.body.Load(); cached != nil {
		return cached.(*block.Body), nil
	}
	b, err := rb.raw.DecodeBody()
	if err != nil {
		return nil, err
	}
	rb.body.Store(b)
	return b, nil
}

func (rb *rawBlock) Block() (*block.Block, error) {
	if cached := rb.block.Load(); cached != nil {
		return cached.(*block.Block), nil
	}

	// h, err := rb.Header()
	// if err != nil {
	// 	fmt.Println("decode header error")
	// 	return nil, err
	// }

	// if h.Number() == 0 {
	// 	b, err := rb.Body()
	// 	if err != nil {
	// 		return nil, err
	// 	}

	// 	block := block.Compose(h, b.Txs)

	// 	rb.block.Store(block)
	// 	return block, nil
	// } else {
	blk, err := block.BlockDecodeFromBytes(rb.raw)
	if err != nil {
		return nil, err
		// panic("load block failed" + err.Error())
	}
	rb.block.Store(blk)
	rb.header.Store(blk.Header())
	rb.body.Store(blk.Body())
	return blk, nil
	// }
	/************
	h, err := rb.Header()
	if err != nil {
		return nil, err
	}
	b, err := rb.Body()
	if err != nil {
		return nil, err
	}

	block := block.Compose(h, b.Txs)

	rb.block.Store(block)
	return block, nil
	*************/
}
