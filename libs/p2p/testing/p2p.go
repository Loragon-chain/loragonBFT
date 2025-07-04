// Package testing includes useful utilities for mocking
// a beacon node's p2p service for unit tests.
package testing

import (
	"bytes"
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Loragon-chain/loragonBFT/libs/p2p/encoder"
	"github.com/Loragon-chain/loragonBFT/libs/p2p/peers"
	"github.com/Loragon-chain/loragonBFT/libs/p2p/peers/scorers"
	ethpb "github.com/OffchainLabs/prysm/v6/proto/prysm/v1alpha1"
	"github.com/OffchainLabs/prysm/v6/proto/prysm/v1alpha1/metadata"
	"github.com/OffchainLabs/prysm/v6/testing/require"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/enr"
	"github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/config"
	core "github.com/libp2p/go-libp2p/core"
	"github.com/libp2p/go-libp2p/core/control"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	"github.com/multiformats/go-multiaddr"
	ssz "github.com/prysmaticlabs/fastssz"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
)

// We have to declare this again here to prevent a circular dependency
// with the main p2p package.
const (
	metadataV1Topic = "/eth2/beacon_chain/req/metadata/1"
	metadataV2Topic = "/eth2/beacon_chain/req/metadata/2"
	metadataV3Topic = "/eth2/beacon_chain/req/metadata/3"
)

// TestP2P represents a p2p implementation that can be used for testing.
type TestP2P struct {
	t               *testing.T
	BHost           host.Host
	EnodeID         enode.ID
	pubsub          *pubsub.PubSub
	joinedTopics    map[string]*pubsub.Topic
	BroadcastCalled atomic.Bool
	DelaySend       bool
	Digest          [4]byte
	peers           *peers.Status
	LocalMetadata   metadata.Metadata
}

// NewTestP2P initializes a new p2p test service.
func NewTestP2P(t *testing.T, userOptions ...config.Option) *TestP2P {
	ctx := context.Background()
	options := []config.Option{
		libp2p.ResourceManager(&network.NullResourceManager{}),
		libp2p.Transport(tcp.NewTCPTransport),
		libp2p.DefaultListenAddrs,
	}

	options = append(options, userOptions...)

	h, err := libp2p.New(options...)
	require.NoError(t, err)
	ps, err := pubsub.NewFloodSub(ctx, h,
		pubsub.WithMessageSigning(false),
		pubsub.WithStrictSignatureVerification(false),
	)
	if err != nil {
		t.Fatal(err)
	}

	peerStatuses := peers.NewStatus(context.Background(), &peers.StatusConfig{
		PeerLimit: 30,
		ScorerParams: &scorers.Config{
			BadResponsesScorerConfig: &scorers.BadResponsesScorerConfig{
				Threshold: 5,
			},
		},
	})
	return &TestP2P{
		t:            t,
		BHost:        h,
		pubsub:       ps,
		joinedTopics: map[string]*pubsub.Topic{},
		peers:        peerStatuses,
	}
}

// Connect two test peers together.
func (p *TestP2P) Connect(b *TestP2P) {
	if err := connect(p.BHost, b.BHost); err != nil {
		p.t.Fatal(err)
	}
}

func connect(a, b host.Host) error {
	pinfo := peer.AddrInfo{
		ID:    b.ID(),
		Addrs: b.Addrs(),
	}
	return a.Connect(context.Background(), pinfo)
}

// ReceiveRPC simulates an incoming RPC.
func (p *TestP2P) ReceiveRPC(topic string, msg proto.Message) {
	h, err := libp2p.New(libp2p.ResourceManager(&network.NullResourceManager{}))
	require.NoError(p.t, err)
	if err := connect(h, p.BHost); err != nil {
		p.t.Fatalf("Failed to connect two peers for RPC: %v", err)
	}
	s, err := h.NewStream(context.Background(), p.BHost.ID(), protocol.ID(topic+p.Encoding().ProtocolSuffix()))
	if err != nil {
		p.t.Fatalf("Failed to open stream %v", err)
	}

	// Modify defer function to ensure no duplicated close after CloseWrite
	closeWriteDone := false
	defer func() {
		if !closeWriteDone {
			// If CloseWrite was not successfully called, try to close the entire stream
			if err := s.Close(); err != nil {
				p.t.Log(err)
			}
		}
	}()

	castedMsg, ok := msg.(ssz.Marshaler)
	if !ok {
		p.t.Fatalf("%T doesn't support ssz marshaler", msg)
	}
	n, err := p.Encoding().EncodeWithMaxLength(s, castedMsg)
	if err != nil {
		if resetErr := s.Reset(); resetErr != nil {
			p.t.Logf("Failed to reset stream after encoding error: %v", resetErr)
		}
		p.t.Fatalf("Failed to encode message: %v", err)
	}

	if err := s.CloseWrite(); err != nil {
		if resetErr := s.Reset(); resetErr != nil {
			p.t.Logf("Failed to reset stream after CloseWrite error: %v", resetErr)
		}
		p.t.Fatalf("Failed to close write: %v", err)
	}
	closeWriteDone = true

	p.t.Logf("Wrote %d bytes", n)
}

// ReceivePubSub simulates an incoming message over pubsub on a given topic.
func (p *TestP2P) ReceivePubSub(topic string, msg proto.Message) {
	h, err := libp2p.New(libp2p.ResourceManager(&network.NullResourceManager{}))
	require.NoError(p.t, err)
	ps, err := pubsub.NewFloodSub(context.Background(), h,
		pubsub.WithMessageSigning(false),
		pubsub.WithStrictSignatureVerification(false),
	)
	if err != nil {
		p.t.Fatalf("Failed to create flood sub: %v", err)
	}
	if err := connect(h, p.BHost); err != nil {
		p.t.Fatalf("Failed to connect two peers for RPC: %v", err)
	}

	// PubSub requires some delay after connecting for the (*PubSub).processLoop method to
	// pick up the newly connected peer.
	time.Sleep(time.Millisecond * 100)

	castedMsg, ok := msg.(ssz.Marshaler)
	if !ok {
		p.t.Fatalf("%T doesn't support ssz marshaler", msg)
	}
	buf := new(bytes.Buffer)
	if _, err := p.Encoding().EncodeGossip(buf, castedMsg); err != nil {
		p.t.Fatalf("Failed to encode message: %v", err)
	}
	digest, err := p.ForkDigest()
	if err != nil {
		p.t.Fatal(err)
	}
	topicHandle, err := ps.Join(fmt.Sprintf(topic, digest) + p.Encoding().ProtocolSuffix())
	if err != nil {
		p.t.Fatal(err)
	}
	if err := topicHandle.Publish(context.TODO(), buf.Bytes()); err != nil {
		p.t.Fatalf("Failed to publish message; %v", err)
	}
}

// Broadcast a message.
func (p *TestP2P) Broadcast(_ context.Context, _ proto.Message) error {
	p.BroadcastCalled.Store(true)
	return nil
}

// BroadcastAttestation broadcasts an attestation.
func (p *TestP2P) BroadcastAttestation(_ context.Context, _ uint64, _ ethpb.Att) error {
	p.BroadcastCalled.Store(true)
	return nil
}

// BroadcastSyncCommitteeMessage broadcasts a sync committee message.
func (p *TestP2P) BroadcastSyncCommitteeMessage(_ context.Context, _ uint64, _ *ethpb.SyncCommitteeMessage) error {
	p.BroadcastCalled.Store(true)
	return nil
}

// BroadcastBlob broadcasts a blob for mock.
func (p *TestP2P) BroadcastBlob(context.Context, uint64, *ethpb.BlobSidecar) error {
	p.BroadcastCalled.Store(true)
	return nil
}

// SetStreamHandler for RPC.
func (p *TestP2P) SetStreamHandler(topic string, handler network.StreamHandler) {
	p.BHost.SetStreamHandler(protocol.ID(topic), handler)
}

// JoinTopic will join PubSub topic, if not already joined.
func (p *TestP2P) JoinTopic(topic string, opts ...pubsub.TopicOpt) (*pubsub.Topic, error) {
	if _, ok := p.joinedTopics[topic]; !ok {
		joinedTopic, err := p.pubsub.Join(topic, opts...)
		if err != nil {
			return nil, err
		}
		p.joinedTopics[topic] = joinedTopic
	}

	return p.joinedTopics[topic], nil
}

// PublishToTopic publishes message to previously joined topic.
func (p *TestP2P) PublishToTopic(ctx context.Context, topic string, data []byte, opts ...pubsub.PubOpt) error {
	joinedTopic, err := p.JoinTopic(topic)
	if err != nil {
		return err
	}
	return joinedTopic.Publish(ctx, data, opts...)
}

// SubscribeToTopic joins (if necessary) and subscribes to PubSub topic.
func (p *TestP2P) SubscribeToTopic(topic string, opts ...pubsub.SubOpt) (*pubsub.Subscription, error) {
	joinedTopic, err := p.JoinTopic(topic)
	if err != nil {
		return nil, err
	}
	return joinedTopic.Subscribe(opts...)
}

// LeaveTopic closes topic and removes corresponding handler from list of joined topics.
// This method will return error if there are outstanding event handlers or subscriptions.
func (p *TestP2P) LeaveTopic(topic string) error {
	if t, ok := p.joinedTopics[topic]; ok {
		if err := t.Close(); err != nil {
			return err
		}
		delete(p.joinedTopics, topic)
	}
	return nil
}

// Encoding returns ssz encoding.
func (*TestP2P) Encoding() encoder.NetworkEncoding {
	return &encoder.SszNetworkEncoder{}
}

// PubSub returns reference underlying floodsub. This test library uses floodsub
// to ensure all connected peers receive the message.
func (p *TestP2P) PubSub() *pubsub.PubSub {
	return p.pubsub
}

// Disconnect from a peer.
func (p *TestP2P) Disconnect(pid peer.ID) error {
	return p.BHost.Network().ClosePeer(pid)
}

// PeerID returns the Peer ID of the local peer.
func (p *TestP2P) PeerID() peer.ID {
	return p.BHost.ID()
}

// Host returns the libp2p host of the
// local peer.
func (p *TestP2P) Host() host.Host {
	return p.BHost
}

// ENR returns the enr of the local peer.
func (*TestP2P) ENR() *enr.Record {
	return new(enr.Record)
}

// NodeID returns the node id of the local peer.
func (p *TestP2P) NodeID() enode.ID {
	return p.EnodeID
}

// DiscoveryAddresses --
func (*TestP2P) DiscoveryAddresses() ([]multiaddr.Multiaddr, error) {
	return nil, nil
}

// AddConnectionHandler handles the connection with a newly connected peer.
func (p *TestP2P) AddConnectionHandler(f, _ func(ctx context.Context, id peer.ID) error) {
	p.BHost.Network().Notify(&network.NotifyBundle{
		ConnectedF: func(_ network.Network, conn network.Conn) {
			// Must be handled in a goroutine as this callback cannot be blocking.
			go func() {
				p.peers.Add(new(enr.Record), conn.RemotePeer(), conn.RemoteMultiaddr(), conn.Stat().Direction)
				ctx := context.Background()

				p.peers.SetConnectionState(conn.RemotePeer(), peers.Connecting)
				if err := f(ctx, conn.RemotePeer()); err != nil {
					logrus.WithError(err).Error("Could not send successful hello rpc request")
					if err := p.Disconnect(conn.RemotePeer()); err != nil {
						logrus.WithError(err).Errorf("Unable to close peer %s", conn.RemotePeer())
					}
					p.peers.SetConnectionState(conn.RemotePeer(), peers.Disconnected)
					return
				}
				p.peers.SetConnectionState(conn.RemotePeer(), peers.Connected)
			}()
		},
	})
}

// AddDisconnectionHandler --
func (p *TestP2P) AddDisconnectionHandler(f func(ctx context.Context, id peer.ID) error) {
	p.BHost.Network().Notify(&network.NotifyBundle{
		DisconnectedF: func(_ network.Network, conn network.Conn) {
			// Must be handled in a goroutine as this callback cannot be blocking.
			go func() {
				p.peers.SetConnectionState(conn.RemotePeer(), peers.Disconnecting)
				if err := f(context.Background(), conn.RemotePeer()); err != nil {
					logrus.WithError(err).Debug("Unable to invoke callback")
				}
				p.peers.SetConnectionState(conn.RemotePeer(), peers.Disconnected)
			}()
		},
	})
}

// Send a message to a specific peer.
func (p *TestP2P) Send(ctx context.Context, msg interface{}, topic string, pid peer.ID) (network.Stream, error) {
	metadataTopics := map[string]bool{metadataV1Topic: true, metadataV2Topic: true, metadataV3Topic: true}

	t := topic
	if t == "" {
		return nil, fmt.Errorf("protocol doesn't exist for proto message: %v", msg)
	}
	stream, err := p.BHost.NewStream(ctx, pid, core.ProtocolID(t+p.Encoding().ProtocolSuffix()))
	if err != nil {
		return nil, err
	}

	// Reset stream on error to prevent resource leaks
	resetOnError := func(err error) error {
		if err != nil {
			// Try to reset the stream, ignore any reset errors
			if resetErr := stream.Reset(); resetErr != nil {
				p.t.Logf("Failed to reset stream: %v", resetErr)
			}
			return err
		}
		return nil
	}

	if !metadataTopics[topic] {
		castedMsg, ok := msg.(ssz.Marshaler)
		if !ok {
			return nil, resetOnError(fmt.Errorf("%T doesn't support ssz marshaler", msg))
		}
		if _, err := p.Encoding().EncodeWithMaxLength(stream, castedMsg); err != nil {
			return nil, resetOnError(err)
		}
	}

	// Close stream for writing.
	if err := stream.CloseWrite(); err != nil {
		return nil, resetOnError(err)
	}

	// Delay returning the stream for testing purposes
	if p.DelaySend {
		time.Sleep(1 * time.Second)
	}

	return stream, nil
}

// Started always returns true.
func (*TestP2P) Started() bool {
	return true
}

// Peers returns the peer status.
func (p *TestP2P) Peers() *peers.Status {
	return p.peers
}

// FindPeersWithSubnet mocks the p2p func.
func (*TestP2P) FindPeersWithSubnet(_ context.Context, _ string, _ uint64, _ int) (bool, error) {
	return false, nil
}

// RefreshPersistentSubnets mocks the p2p func.
func (*TestP2P) RefreshPersistentSubnets() {}

// ForkDigest mocks the p2p func.
func (p *TestP2P) ForkDigest() ([4]byte, error) {
	return p.Digest, nil
}

// Metadata mocks the peer's metadata.
func (p *TestP2P) Metadata() metadata.Metadata {
	return p.LocalMetadata.Copy()
}

// MetadataSeq mocks metadata sequence number.
func (p *TestP2P) MetadataSeq() uint64 {
	return p.LocalMetadata.SequenceNumber()
}

// AddPingMethod mocks the p2p func.
func (*TestP2P) AddPingMethod(_ func(ctx context.Context, id peer.ID) error) {
	// no-op
}

// InterceptPeerDial .
func (*TestP2P) InterceptPeerDial(peer.ID) (allow bool) {
	return true
}

// InterceptAddrDial .
func (*TestP2P) InterceptAddrDial(peer.ID, multiaddr.Multiaddr) (allow bool) {
	return true
}

// InterceptAccept .
func (*TestP2P) InterceptAccept(_ network.ConnMultiaddrs) (allow bool) {
	return true
}

// InterceptSecured .
func (*TestP2P) InterceptSecured(network.Direction, peer.ID, network.ConnMultiaddrs) (allow bool) {
	return true
}

// InterceptUpgraded .
func (*TestP2P) InterceptUpgraded(network.Conn) (allow bool, reason control.DisconnectReason) {
	return true, 0
}
