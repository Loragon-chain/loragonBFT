// // Copyright (c) 2020 The Meter.io developers

// // Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// // file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>
package node_test

// import (
// 	"context"
// 	"encoding/json"
// 	"io"
// 	"net/http"
// 	"net/http/httptest"
// 	"testing"
// 	"time"

// 	"github.com/Loragon-chain/loragonBFT/api/node"
// 	"github.com/Loragon-chain/loragonBFT/chain"
// 	"github.com/Loragon-chain/loragonBFT/genesis"
// 	"github.com/Loragon-chain/loragonBFT/libs/comm"
// 	"github.com/Loragon-chain/loragonBFT/libs/lvldb"
// 	"github.com/Loragon-chain/loragonBFT/txpool"
// 	"github.com/gorilla/mux"
// 	"github.com/stretchr/testify/assert"
// )

// var ts *httptest.Server

// func TestNode(t *testing.T) {
// 	initCommServer(t)
// 	res := httpGet(t, ts.URL+"/node/network/peers")
// 	var peersStats map[string]string
// 	if err := json.Unmarshal(res, &peersStats); err != nil {
// 		t.Fatal(err)
// 	}
// 	assert.Equal(t, 0, len(peersStats), "count should be zero")
// }

// func initCommServer(t *testing.T) {
// 	db, _ := lvldb.NewMem()
// 	gene := genesis.NewDevnet()

// 	b, err := gene.Build()
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	chain, _ := chain.New(db, b, gene.ValidatorSet(), false)
// 	comm := comm.New(context.Background(), chain, txpool.New(chain, txpool.Options{
// 		Limit:           10000,
// 		LimitPerAccount: 16,
// 		MaxLifetime:     10 * time.Minute,
// 	}), "main", [4]byte{1, 2, 3, 4})
// 	router := mux.NewRouter()
// 	node.NewNode(comm).Mount(router, "/node")
// 	ts = httptest.NewServer(router)
// }

// func httpGet(t *testing.T, url string) []byte {
// 	res, err := http.Get(url)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	r, err := io.ReadAll(res.Body)
// 	res.Body.Close()
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	return r
// }
