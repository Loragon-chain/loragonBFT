package scorers_test

import (
	"context"
	"sort"
	"testing"

	"github.com/Loragon-chain/loragonBFT/libs/p2p/peers"
	"github.com/Loragon-chain/loragonBFT/libs/p2p/peers/peerdata"
	"github.com/Loragon-chain/loragonBFT/libs/p2p/peers/scorers"
	"github.com/OffchainLabs/prysm/v6/testing/assert"
	"github.com/OffchainLabs/prysm/v6/testing/require"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

func TestScorers_BadResponses_Score(t *testing.T) {
	const pid = "peer1"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	peerStatuses := peers.NewStatus(ctx, &peers.StatusConfig{
		PeerLimit: 30,
		ScorerParams: &scorers.Config{
			BadResponsesScorerConfig: &scorers.BadResponsesScorerConfig{
				Threshold: 4,
			},
		},
	})
	scorer := peerStatuses.Scorers().BadResponsesScorer()

	assert.Equal(t, 0., scorer.Score(pid), "Unexpected score for unregistered peer")

	scorer.Increment(pid)
	assert.NoError(t, scorer.IsBadPeer(pid))
	assert.Equal(t, -2.5, scorer.Score(pid))

	scorer.Increment(pid)
	assert.NoError(t, scorer.IsBadPeer(pid))
	assert.Equal(t, float64(-5), scorer.Score(pid))

	scorer.Increment(pid)
	assert.NoError(t, scorer.IsBadPeer(pid))
	assert.Equal(t, float64(-7.5), scorer.Score(pid))

	scorer.Increment(pid)
	assert.NotNil(t, scorer.IsBadPeer(pid))
	assert.Equal(t, -100.0, scorer.Score(pid))
}

func TestScorers_BadResponses_ParamsThreshold(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	maxBadResponses := 2
	peerStatuses := peers.NewStatus(ctx, &peers.StatusConfig{
		PeerLimit: 30,
		ScorerParams: &scorers.Config{
			BadResponsesScorerConfig: &scorers.BadResponsesScorerConfig{
				Threshold: maxBadResponses,
			},
		},
	})
	scorer := peerStatuses.Scorers()
	assert.Equal(t, maxBadResponses, scorer.BadResponsesScorer().Params().Threshold)
}

func TestScorers_BadResponses_Count(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	peerStatuses := peers.NewStatus(ctx, &peers.StatusConfig{
		PeerLimit:    30,
		ScorerParams: &scorers.Config{},
	})
	scorer := peerStatuses.Scorers()

	pid := peer.ID("peer1")
	_, err := scorer.BadResponsesScorer().Count(pid)
	assert.ErrorContains(t, peerdata.ErrPeerUnknown.Error(), err)

	peerStatuses.Add(nil, pid, nil, network.DirUnknown)
	count, err := scorer.BadResponsesScorer().Count(pid)
	assert.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestScorers_BadResponses_Decay(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	maxBadResponses := 2
	peerStatuses := peers.NewStatus(ctx, &peers.StatusConfig{
		PeerLimit: 30,
		ScorerParams: &scorers.Config{
			BadResponsesScorerConfig: &scorers.BadResponsesScorerConfig{
				Threshold: maxBadResponses,
			},
		},
	})
	scorer := peerStatuses.Scorers().BadResponsesScorer()

	// Peer 1 has 0 bad responses.
	pid1 := peer.ID("peer1")
	peerStatuses.Add(nil, pid1, nil, network.DirUnknown)
	badResponses, err := scorer.Count(pid1)
	require.NoError(t, err)
	assert.Equal(t, 0, badResponses)

	// Peer 2 has 1 bad response.
	pid2 := peer.ID("peer2")
	peerStatuses.Add(nil, pid2, nil, network.DirUnknown)
	scorer.Increment(pid2)
	badResponses, err = scorer.Count(pid2)
	require.NoError(t, err)
	assert.Equal(t, 1, badResponses)

	// Peer 3 has 2 bad response.
	pid3 := peer.ID("peer3")
	peerStatuses.Add(nil, pid3, nil, network.DirUnknown)
	scorer.Increment(pid3)
	scorer.Increment(pid3)
	badResponses, err = scorer.Count(pid3)
	require.NoError(t, err)
	assert.Equal(t, 2, badResponses)

	// Decay the values
	scorer.Decay()

	// Ensure the new values are as expected
	badResponses, err = scorer.Count(pid1)
	require.NoError(t, err)
	assert.Equal(t, 0, badResponses, "unexpected bad responses for pid1")

	badResponses, err = scorer.Count(pid2)
	require.NoError(t, err)
	assert.Equal(t, 0, badResponses, "unexpected bad responses for pid2")

	badResponses, err = scorer.Count(pid3)
	require.NoError(t, err)
	assert.Equal(t, 1, badResponses, "unexpected bad responses for pid3")
}

func TestScorers_BadResponses_IsBadPeer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	peerStatuses := peers.NewStatus(ctx, &peers.StatusConfig{
		PeerLimit:    30,
		ScorerParams: &scorers.Config{},
	})
	scorer := peerStatuses.Scorers().BadResponsesScorer()
	pid := peer.ID("peer1")
	assert.NoError(t, scorer.IsBadPeer(pid))

	peerStatuses.Add(nil, pid, nil, network.DirUnknown)
	assert.NoError(t, scorer.IsBadPeer(pid))

	for i := 0; i < scorers.DefaultBadResponsesThreshold; i++ {
		scorer.Increment(pid)
		if i == scorers.DefaultBadResponsesThreshold-1 {
			assert.NotNil(t, scorer.IsBadPeer(pid), "Unexpected peer status")
		} else {
			assert.NoError(t, scorer.IsBadPeer(pid), "Unexpected peer status")
		}
	}
}

func TestScorers_BadResponses_BadPeers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	peerStatuses := peers.NewStatus(ctx, &peers.StatusConfig{
		PeerLimit:    30,
		ScorerParams: &scorers.Config{},
	})
	scorer := peerStatuses.Scorers().BadResponsesScorer()
	pids := []peer.ID{peer.ID("peer1"), peer.ID("peer2"), peer.ID("peer3"), peer.ID("peer4"), peer.ID("peer5")}
	for i := 0; i < len(pids); i++ {
		peerStatuses.Add(nil, pids[i], nil, network.DirUnknown)
	}
	for i := 0; i < scorers.DefaultBadResponsesThreshold; i++ {
		scorer.Increment(pids[1])
		scorer.Increment(pids[2])
		scorer.Increment(pids[4])
	}
	assert.NoError(t, scorer.IsBadPeer(pids[0]), "Invalid peer status")
	assert.NotNil(t, scorer.IsBadPeer(pids[1]), "Invalid peer status")
	assert.NotNil(t, scorer.IsBadPeer(pids[2]), "Invalid peer status")
	assert.NoError(t, scorer.IsBadPeer(pids[3]), "Invalid peer status")
	assert.NotNil(t, scorer.IsBadPeer(pids[4]), "Invalid peer status")
	want := []peer.ID{pids[1], pids[2], pids[4]}
	badPeers := scorer.BadPeers()
	sort.Slice(badPeers, func(i, j int) bool {
		return badPeers[i] < badPeers[j]
	})
	assert.DeepEqual(t, want, badPeers, "Unexpected list of bad peers")
}
