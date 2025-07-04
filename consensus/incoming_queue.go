package consensus

import (
	"encoding/hex"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/Loragon-chain/loragonBFT/block"
	"github.com/Loragon-chain/loragonBFT/types"
	"github.com/ethereum/go-ethereum/common"
	lru "github.com/hashicorp/golang-lru"
)

const (
	IN_QUEUE_TTL = time.Second * 5
)

type IncomingMsg struct {
	//Msg    block.ConsensusMessage
	// from envelope
	Msg        block.ConsensusMessage
	SenderAddr common.Address

	// extras
	Hash         types.Bytes32
	ShortHashStr string
	EnqueueAt    time.Time
	ExpireAt     time.Time
	ProcessCount uint32
}

func newIncomingMsg(msg block.ConsensusMessage, senderAddr common.Address) *IncomingMsg {
	msgHash := msg.GetMsgHash()
	// slog.Info("Incoming Msg", "msg", msg.String(), "msgHash", hex.EncodeToString(msgHash[:]), "rawData", hex.EncodeToString(rawData))
	shortMsgHash := hex.EncodeToString(msgHash[:])[:8]
	return &IncomingMsg{
		Msg:          msg,
		SenderAddr:   senderAddr,
		Hash:         msgHash,
		ShortHashStr: shortMsgHash,

		ProcessCount: 0,
	}
}

func (m *IncomingMsg) Expired() bool {
	// return time.Now().After(m.ExpireAt)
	return false
}

type IncomingQueue struct {
	sync.Mutex
	logger *slog.Logger
	queue  chan (IncomingMsg)
	cache  *lru.Cache
}

func NewIncomingQueue() *IncomingQueue {
	cache, err := lru.New(1024)
	if err != nil {
		panic("could not create cache")
	}
	return &IncomingQueue{
		logger: slog.With("pkg", "in"),
		queue:  make(chan (IncomingMsg), 1024),
		cache:  cache,
	}
}

func (q *IncomingQueue) forceAdd(mi IncomingMsg) {
	defer q.Mutex.Unlock()
	q.Mutex.Lock()

	for len(q.queue) >= cap(q.queue) {
		dropped := <-q.queue
		q.logger.Warn(fmt.Sprintf("dropped %s due to cap", dropped.Msg.String())) //, "from", dropped.Peer)
	}

	q.queue <- mi
}

func (q *IncomingQueue) DelayedAdd(mi IncomingMsg) {
	mi.ProcessCount = mi.ProcessCount + 1
	time.AfterFunc(time.Second, func() {
		q.forceAdd(mi)
	})
}

func (q *IncomingQueue) Add(mi IncomingMsg) error {
	defer q.Mutex.Unlock()
	q.Mutex.Lock()
	if q.cache.Contains(mi.Hash) {
		return ErrKnownMsg
	}
	q.cache.Add(mi.Hash, true)

	// instead of drop the latest message, drop the oldest one in front of queue
	for len(q.queue) >= cap(q.queue) {
		dropped := <-q.queue
		q.logger.Warn(fmt.Sprintf("dropped %s due to cap", dropped.Msg.String()), "from", dropped.SenderAddr.String())
	}

	q.logger.Info(fmt.Sprintf(`recv %s`, mi.Msg.String()), "from", mi.SenderAddr, "qlen", len(q.queue))
	mi.EnqueueAt = time.Now()
	mi.ExpireAt = time.Now().Add(IN_QUEUE_TTL)
	q.queue <- mi
	return nil
}

func (q *IncomingQueue) drain() {
	defer q.Mutex.Unlock()
	q.Mutex.Lock()
	for len(q.queue) > 0 {
		<-q.queue
	}
}

func (q *IncomingQueue) Queue() chan (IncomingMsg) {
	return q.queue
}

func (q *IncomingQueue) Len() int {
	return len(q.queue)
}
