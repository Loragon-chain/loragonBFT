package node

import (
	"encoding/hex"

	ctypes "github.com/cometbft/cometbft/rpc/core/types"
	rpctypes "github.com/cometbft/cometbft/rpc/jsonrpc/types"
	"github.com/cometbft/cometbft/types"
)

func (n *Node) BroadcastTxSync(ctx *rpctypes.Context, tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	// res, err := n.proxyApp.Mempool().CheckTx(ctx.Context(), &abci.CheckTxRequest{
	// 	Tx:   tx,
	// 	Type: v1.CHECK_TX_TYPE_CHECK,
	// })
	// if err != nil {
	// 	return nil, err
	// }
	// n.logger.Info("BroadcastTxSync", "hash", tx.Hash(), "result", res.String())
	// fmt.Println("BroadcastTxSync", "hash", tx.Hash(), "result", res.String())

	// pid, err := peer.IDFromBytes(n.nodeKey.PrivKey.Bytes())
	// if err != nil {
	// 	n.logger.Error("BroadcastTxSync IDFromBytes", "error", err)
	// 	return nil, err
	// }

	// err = n.rpc.NotifyNewTx(pid, tx)
	err := n.txPool.StrictlyAdd(tx)
	if err != nil {
		n.logger.Error("BroadcastTxSync NotifyNewTx", "error", err)
		return nil, err
	}
	n.logger.Info("BroadcastTxSync", "hash", hex.EncodeToString(tx.Hash()))

	return &ctypes.ResultBroadcastTx{
		Code: 0,
		// Data:      res.Data,
		// Log:       res.Log,
		// Codespace: res.Codespace,
		Hash: tx.Hash(),
	}, nil
}

func (n *Node) BroadcastTxAsync(ctx *rpctypes.Context, tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	return n.BroadcastTxSync(ctx, tx)
}

func (n *Node) Tx(ctx *rpctypes.Context, hash []byte, prove bool) (*ctypes.ResultTx, error) {
	// if _, ok := env.TxIndexer.(*null.TxIndex); ok {
	// 	return nil, errors.New("transaction indexing is disabled")
	// }

	// r, err := n.rpc.Get(hash)
	// if err != nil {
	// 	return nil, err
	// }

	// if r == nil {
	// 	return nil, fmt.Errorf("tx (%X) not found", hash)
	// }

	// var proof types.TxProof
	// if prove {
	// 	block, _ := env.BlockStore.LoadBlock(r.Height)
	// 	if block != nil {
	// 		proof = block.Data.Txs.Proof(int(r.Index))
	// 	}
	// }

	// return &ctypes.ResultTx{
	// 	Hash:     hash,
	// 	Height:   r.Height,
	// 	Index:    r.Index,
	// 	TxResult: r.Result,
	// 	Tx:       r.Tx,
	// 	Proof:    proof,
	// }, nil
	return nil, nil
}
