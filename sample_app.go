package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/cockroachdb/pebble"
	"github.com/cometbft/cometbft/abci/types"
	abcitypes "github.com/cometbft/cometbft/abci/types"
)

var (
	stateKey        = []byte("stateKey")
	kvPairPrefixKey = []byte("kvPairKey:")
)

const (
	ValidatorPrefix        = "val="
	AppVersion      uint64 = 1
	defaultLane     string = "default"
)

type State struct {
	db *pebble.DB
	// Size is essentially the amount of transactions that have been processes.
	// This is used for the appHash
	Size   int64 `json:"size"`
	Height int64 `json:"height"`
}

// Application is the kvstore state machine. It complies with the abci.Application interface.
// It takes transactions in the form of key=value and saves them in a database. This is
// a somewhat trivial example as there is no real state execution.
type KVStoreApplication struct {
	db           *pebble.DB
	onGoingBatch *pebble.Batch
	logger       log.Logger
}

var _ abcitypes.Application = (*KVStoreApplication)(nil)

func newDB(dbDir string) *pebble.DB {
	db, err := pebble.Open(dbDir, nil)
	if err != nil {
		panic(fmt.Errorf("failed to create persistent app at %s: %w", dbDir, err))
	}
	return db
}

func NewKVStoreApplication(dbDir string) *KVStoreApplication {
	return NewApplication(newDB(dbDir))
}

func NewApplication(db *pebble.DB) *KVStoreApplication {
	return &KVStoreApplication{
		db: db,
	}
}

func loadState(db *pebble.DB) State {
	var state State
	state.db = db
	stateBytes, _, err := db.Get(stateKey)
	if err != nil && err != pebble.ErrNotFound {
		panic(err)
	}

	if len(stateBytes) == 0 {
		return state
	}
	err = json.Unmarshal(stateBytes, &state)
	if err != nil {
		panic(err)
	}
	return state
}

func (app *KVStoreApplication) isValid(tx []byte) uint32 {
	// check format
	parts := bytes.Split(tx, []byte("="))
	if len(parts) != 2 {
		return 1
	}
	return 0
}

func (app *KVStoreApplication) Info(_ context.Context, info *abcitypes.InfoRequest) (*abcitypes.InfoResponse, error) {
	return &abcitypes.InfoResponse{}, nil

}

// func (app *KVStoreApplication) getValidators() (validators []abcitypes.ValidatorUpdate) {
// 	itr, err := app.state.db.NewIter(nil)
// 	if err != nil {
// 		panic(err)
// 	}
// 	for ; itr.Valid(); itr.Next() {
// 		if isValidatorTx(itr.Key()) {
// 			validator := new(types.ValidatorUpdate)
// 			err := types.ReadMessage(bytes.NewBuffer(itr.Value()), validator)
// 			if err != nil {
// 				panic(err)
// 			}
// 			validators = append(validators, *validator)
// 		}
// 	}
// 	if err = itr.Error(); err != nil {
// 		panic(err)
// 	}
// 	return validators
// }

func (app *KVStoreApplication) Query(_ context.Context, req *abcitypes.QueryRequest) (*abcitypes.QueryResponse, error) {
	resp := abcitypes.QueryResponse{Key: req.Data}

	value, closer, err := app.db.Get(req.Data)
	if err != nil {
		if errors.Is(err, pebble.ErrNotFound) {
			resp.Log = "key does not exist"
		} else {
			log.Panicf("Error reading database, unable to execute query: %v", err)
		}
		return &resp, nil
	}
	defer closer.Close()

	resp.Log = "exists"
	resp.Value = value
	return &resp, nil
}

func (app *KVStoreApplication) CheckTx(_ context.Context, check *abcitypes.CheckTxRequest) (*abcitypes.CheckTxResponse, error) {
	code := app.isValid(check.Tx)
	return &abcitypes.CheckTxResponse{Code: code}, nil
}

func (app *KVStoreApplication) InitChain(_ context.Context, req *abcitypes.InitChainRequest) (*abcitypes.InitChainResponse, error) {

	appHash := make([]byte, 8)
	return &types.InitChainResponse{
		AppHash:    appHash,
		Validators: req.Validators,
	}, nil
}

func (app *KVStoreApplication) PrepareProposal(ctx context.Context, proposal *abcitypes.PrepareProposalRequest) (*abcitypes.PrepareProposalResponse, error) {
	return &abcitypes.PrepareProposalResponse{Txs: proposal.Txs}, nil
}

func (app *KVStoreApplication) ProcessProposal(ctx context.Context, proposal *abcitypes.ProcessProposalRequest) (*abcitypes.ProcessProposalResponse, error) {
	return &abcitypes.ProcessProposalResponse{Status: abcitypes.PROCESS_PROPOSAL_STATUS_ACCEPT}, nil
}

func (app *KVStoreApplication) FinalizeBlock(_ context.Context, req *abcitypes.FinalizeBlockRequest) (*abcitypes.FinalizeBlockResponse, error) {
	var txs = make([]*abcitypes.ExecTxResult, len(req.Txs))
	var updates = make([]abcitypes.ValidatorUpdate, 0)
	var events = make([]abcitypes.Event, 0)

	app.onGoingBatch = app.db.NewBatch()
	defer app.onGoingBatch.Close()

	for i, tx := range req.Txs {
		if code := app.isValid(tx); code != 0 {
			log.Printf("Error: invalid transaction index %v", i)
			txs[i] = &abcitypes.ExecTxResult{Code: code}
		} else {
			parts := bytes.SplitN(tx, []byte("="), 2)
			key, value := parts[0], parts[1]
			log.Printf("Adding key %s with value %s", key, value)

			if err := app.onGoingBatch.Set(key, value, pebble.Sync); err != nil {
				log.Panicf("Error writing to database, unable to execute tx: %v", err)
			}

			log.Printf("Successfully added key %s with value %s", key, value)

			txs[i] = &abcitypes.ExecTxResult{
				Code: 0,
				Events: []abcitypes.Event{
					{
						Type: "app",
						Attributes: []abcitypes.EventAttribute{
							{Key: "key", Value: string(key), Index: true},
							{Key: "value", Value: string(value), Index: true},
						},
					},
				},
			}
		}
	}

	// loragonPubkeyHex := "b8cff948c0d4022781418e9242b37da09937d901f0af436756b4228099d1ad5772223106282533bb6105723a0bc90b2e"
	// if req.Height == 6 {
	// 	pubkey, _ := hex.DecodeString(loragonPubkeyHex)
	// 	fmt.Println("loragonPubkeyHex", pubkey)
	// 	updates = append(updates, abcitypes.ValidatorUpdate{
	// 		Power:       10,
	// 		PubKeyBytes: pubkey,
	// 		PubKeyType:  "bls12-381.pubkey",
	// 	})
	// 	events = append(events, abcitypes.Event{
	// 		Type: "ValidatorExtra",
	// 		Attributes: []abcitypes.EventAttribute{
	// 			{Key: "pubkey", Value: loragonPubkeyHex},
	// 			{Key: "name", Value: "loragon"},
	// 			{Key: "ip", Value: "42.113.171.94"}, // TODO: change to the actual IP address
	// 			{Key: "port", Value: "8670"},
	// 		},
	// 	})
	// }

	// if req.Height == 20 {
	// 	pubkey, _ := hex.DecodeString(loragonPubkeyHex)
	// 	updates = append(updates, abcitypes.ValidatorUpdate{
	// 		Power:       0,
	// 		PubKeyBytes: pubkey,
	// 		PubKeyType:  "bls12-381.pubkey",
	// 	})
	// }

	if err := app.onGoingBatch.Commit(pebble.Sync); err != nil {
		log.Panicf("Failed to commit batch: %v", err)
	}

	return &abcitypes.FinalizeBlockResponse{
		TxResults:        txs,
		ValidatorUpdates: updates,
		Events:           events,
	}, nil
}

func (app *KVStoreApplication) Commit(_ context.Context, commit *abcitypes.CommitRequest) (*abcitypes.CommitResponse, error) {
	return &abcitypes.CommitResponse{}, nil
}

func (app *KVStoreApplication) ListSnapshots(_ context.Context, snapshots *abcitypes.ListSnapshotsRequest) (*abcitypes.ListSnapshotsResponse, error) {
	return &abcitypes.ListSnapshotsResponse{}, nil
}

func (app *KVStoreApplication) OfferSnapshot(_ context.Context, snapshot *abcitypes.OfferSnapshotRequest) (*abcitypes.OfferSnapshotResponse, error) {
	return &abcitypes.OfferSnapshotResponse{}, nil
}

func (app *KVStoreApplication) LoadSnapshotChunk(_ context.Context, chunk *abcitypes.LoadSnapshotChunkRequest) (*abcitypes.LoadSnapshotChunkResponse, error) {
	return &abcitypes.LoadSnapshotChunkResponse{}, nil
}

func (app *KVStoreApplication) ApplySnapshotChunk(_ context.Context, chunk *abcitypes.ApplySnapshotChunkRequest) (*abcitypes.ApplySnapshotChunkResponse, error) {
	return &abcitypes.ApplySnapshotChunkResponse{Result: abcitypes.APPLY_SNAPSHOT_CHUNK_RESULT_ACCEPT}, nil
}

func (app *KVStoreApplication) ExtendVote(_ context.Context, extend *abcitypes.ExtendVoteRequest) (*abcitypes.ExtendVoteResponse, error) {
	return &abcitypes.ExtendVoteResponse{}, nil
}

func (app *KVStoreApplication) VerifyVoteExtension(_ context.Context, verify *abcitypes.VerifyVoteExtensionRequest) (*abcitypes.VerifyVoteExtensionResponse, error) {
	return &abcitypes.VerifyVoteExtensionResponse{}, nil
}
