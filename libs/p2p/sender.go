package p2p

import (
	"context"
	"fmt"

	snmsg "github.com/Loragon-chain/loragonBFT/libs/message"
	"github.com/OffchainLabs/prysm/v6/monitoring/tracing"
	"github.com/OffchainLabs/prysm/v6/monitoring/tracing/trace"
	"github.com/kr/pretty"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/sirupsen/logrus"
)

// Send a message to a specific peer. The returned stream may be used for reading, but has been
// closed for writing.
//
// When done, the caller must Close or Reset on the stream.
func (s *Service) Send(ctx context.Context, message interface{}, baseTopic string, pid peer.ID) (network.Stream, error) {
	ctx, span := trace.StartSpan(ctx, "p2p.Send")
	defer span.End()
	if err := VerifyTopicMapping(baseTopic, message); err != nil {
		return nil, err
	}
	topic := baseTopic + s.Encoding().ProtocolSuffix()
	span.SetAttributes(trace.StringAttribute("topic", topic))

	log.WithFields(logrus.Fields{
		"topic":   topic,
		"request": pretty.Sprint(message),
	}).Tracef("Sending RPC request to peer %s", pid.String())

	// Apply max dial timeout when opening a new stream.
	ctx, cancel := context.WithTimeout(ctx, maxDialTimeout)
	defer cancel()

	s.logger.Debug(fmt.Sprintf("rpc call %v", message.(*snmsg.RPCEnvelope).MsgType), "toPeer", pid)
	stream, err := s.host.NewStream(ctx, pid, protocol.ID(topic))
	if err != nil {
		tracing.AnnotateError(span, err)
		return nil, err
	}

	// Ensure stream is reset in case of error to prevent resource leak
	resetOnError := func(err error) error {
		if err != nil {
			// Log the error but don't return Reset error - the original error is more important
			if resetErr := stream.Reset(); resetErr != nil {
				s.logger.Debug("Failed to reset stream", "err", resetErr)
			}
			return err
		}
		return nil
	}

	bs, err := message.(*snmsg.RPCEnvelope).MarshalSSZ()
	if err != nil {
		s.logger.Debug("Failed to marshal message", "err", err)
		return nil, resetOnError(err)
	}

	_, err = stream.Write(bs)
	if err != nil {
		s.logger.Debug("Failed to write to stream", "err", err)
		return nil, resetOnError(err)
	}

	// Close stream for writing.
	if err := stream.CloseWrite(); err != nil {
		tracing.AnnotateError(span, err)
		return nil, resetOnError(err)
	}

	return stream, nil
}

func (s *Service) CloseStream(stream network.Stream) {
	if stream == nil {
		return
	}
	stream.Close()
}
