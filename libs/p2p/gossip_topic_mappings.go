package p2p

import (
	"reflect"

	"github.com/Loragon-chain/loragonBFT/libs/message"
	"github.com/OffchainLabs/prysm/v6/config/params"
	"github.com/OffchainLabs/prysm/v6/consensus-types/primitives"
	ethpb "github.com/OffchainLabs/prysm/v6/proto/prysm/v1alpha1"
	"google.golang.org/protobuf/proto"
)

// gossipTopicMappings represent the protocol ID to protobuf message type map for easy
// lookup.
var gossipTopicMappings = map[string]func() proto.Message{

	ConsensusTopic: func() proto.Message { return &message.ConsensusEnvelope{} },
}

// GossipTopicMappings is a function to return the assigned data type
// versioned by epoch.
func GossipTopicMappings(topic string, epoch primitives.Epoch) proto.Message {
	switch topic {
	case BlockSubnetTopicFormat:
		if epoch >= params.BeaconConfig().ElectraForkEpoch {
			return &ethpb.SignedBeaconBlockElectra{}
		}
		if epoch >= params.BeaconConfig().DenebForkEpoch {
			return &ethpb.SignedBeaconBlockDeneb{}
		}
		if epoch >= params.BeaconConfig().CapellaForkEpoch {
			return &ethpb.SignedBeaconBlockCapella{}
		}
		if epoch >= params.BeaconConfig().BellatrixForkEpoch {
			return &ethpb.SignedBeaconBlockBellatrix{}
		}
		if epoch >= params.BeaconConfig().AltairForkEpoch {
			return &ethpb.SignedBeaconBlockAltair{}
		}
		return gossipMessage(topic)
	case AttestationSubnetTopicFormat:
		if epoch >= params.BeaconConfig().ElectraForkEpoch {
			return &ethpb.AttestationElectra{}
		}
		return gossipMessage(topic)
	case AttesterSlashingSubnetTopicFormat:
		if epoch >= params.BeaconConfig().ElectraForkEpoch {
			return &ethpb.AttesterSlashingElectra{}
		}
		return gossipMessage(topic)
	case AggregateAndProofSubnetTopicFormat:
		if epoch >= params.BeaconConfig().ElectraForkEpoch {
			return &ethpb.SignedAggregateAttestationAndProofElectra{}
		}
		return gossipMessage(topic)
	default:
		return gossipMessage(topic)
	}
}

func gossipMessage(topic string) proto.Message {
	msgGen, ok := gossipTopicMappings[topic]
	if !ok {
		return nil
	}
	return msgGen()
}

// AllTopics returns all topics stored in our
// gossip mapping.
func AllTopics() []string {
	var topics []string
	for k := range gossipTopicMappings {
		topics = append(topics, k)
	}
	return topics
}

// GossipTypeMapping is the inverse of GossipTopicMappings so that an arbitrary protobuf message
// can be mapped to a protocol ID string.
var GossipTypeMapping = make(map[reflect.Type]string, len(gossipTopicMappings))

func init() {
	for k, v := range gossipTopicMappings {
		GossipTypeMapping[reflect.TypeOf(v())] = k
	}
	// Specially handle Altair objects.
	GossipTypeMapping[reflect.TypeOf(&ethpb.SignedBeaconBlockAltair{})] = BlockSubnetTopicFormat
	// Specially handle Bellatrix objects.
	GossipTypeMapping[reflect.TypeOf(&ethpb.SignedBeaconBlockBellatrix{})] = BlockSubnetTopicFormat
	// Specially handle Capella objects.
	GossipTypeMapping[reflect.TypeOf(&ethpb.SignedBeaconBlockCapella{})] = BlockSubnetTopicFormat
	// Specially handle Deneb objects.
	GossipTypeMapping[reflect.TypeOf(&ethpb.SignedBeaconBlockDeneb{})] = BlockSubnetTopicFormat
	// Specially handle Electra objects.
	GossipTypeMapping[reflect.TypeOf(&ethpb.SignedBeaconBlockElectra{})] = BlockSubnetTopicFormat
	GossipTypeMapping[reflect.TypeOf(&ethpb.AttestationElectra{})] = AttestationSubnetTopicFormat
	GossipTypeMapping[reflect.TypeOf(&ethpb.AttesterSlashingElectra{})] = AttesterSlashingSubnetTopicFormat
	GossipTypeMapping[reflect.TypeOf(&ethpb.SignedAggregateAttestationAndProofElectra{})] = AggregateAndProofSubnetTopicFormat
}
