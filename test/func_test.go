package test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/p2p/enode"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/assert"
)

func TestParseGenericAddrs(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedEnode []string
		expectedMulti []string
		expectError   bool
	}{
		{
			name:  "valid multiaddr input",
			input: "7cede90b09728cc5b1837568b88860561dab92f6@192.168.10.4:26656,b5fc278c1c2e3181074627e35f96a4ce3d732d10@192.168.10.3:26656",
			expectedMulti: []string{
				"/ip4/192.168.10.4/tcp/26656",
				"/ip4/192.168.10.3/tcp/26656",
			},
			expectError: false,
		},
		{
			name:          "empty input",
			input:         "",
			expectedMulti: []string{},
			expectError:   false,
		},
		{
			name:          "invalid format",
			input:         "invalid@format",
			expectedMulti: []string{},
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var bootstrapNodes []string
			if tt.input != "" {
				splitAddrs := strings.Split(tt.input, ",")
				for _, splitAddr := range splitAddrs {
					ipPorts := strings.Split(splitAddr, "@")
					if len(ipPorts) != 2 {
						if tt.expectError {
							return
						}
						t.Fatalf("invalid address format: %s", splitAddr)
					}

					ipPort := strings.Split(ipPorts[1], ":")
					if len(ipPort) != 2 {
						if tt.expectError {
							return
						}
						t.Fatalf("invalid ip:port format: %s", ipPorts[1])
					}

					formattedNodeAddr := fmt.Sprintf("/ip4/%s/tcp/%s", ipPort[0], ipPort[1])
					t.Log("formattedNodeAddr:", "formattedNodeAddr", formattedNodeAddr)
					bootstrapNodes = append(bootstrapNodes, formattedNodeAddr)

					t.Log("bootstrapNodes:", "bootstrapNodes", bootstrapNodes)
				}
			}

			peers, err := PeersFromStringAddrs(t, bootstrapNodes)
			if err != nil {
				t.Fatalf("error parsing peers: %v", err)
			}

			t.Log("peers:", "peers", peers)

			if !tt.expectError {
				assert.Equal(t, len(tt.expectedMulti), len(bootstrapNodes), "multiAddr count mismatch")
				for i, addr := range bootstrapNodes {
					assert.Equal(t, tt.expectedMulti[i], addr, "multiAddr[%d] mismatch", i)
				}
			}
		})
	}
}

func PeersFromStringAddrs(t *testing.T, addrs []string) ([]ma.Multiaddr, error) {
	var allAddrs []ma.Multiaddr
	enodeString, multiAddrString := parseGenericAddrs(addrs)
	t.Log("enodeString:", "enodeString", enodeString)
	t.Log("multiAddrString:", "multiAddrString", multiAddrString)
	for _, stringAddr := range multiAddrString {
		addr, err := multiAddrFromString(stringAddr)
		if err != nil {
			panic(err)
		}
		allAddrs = append(allAddrs, addr)
	}
	// for _, stringAddr := range enodeString {
	// 	enodeAddr, err := enode.Parse(enode.ValidSchemes, stringAddr)
	// 	if err != nil {
	// 		return nil, errors.Wrapf(err, "Could not get enode from string")
	// 	}
	// 	nodeAddrs, err := retrieveMultiAddrsFromNode(enodeAddr)
	// 	if err != nil {
	// 		return nil, errors.Wrapf(err, "Could not get multiaddr")
	// 	}
	// 	allAddrs = append(allAddrs, nodeAddrs...)
	// }
	return allAddrs, nil
}

func parseGenericAddrs(addrs []string) (enodeString, multiAddrString []string) {
	for _, addr := range addrs {
		if addr == "" {
			// Ignore empty entries
			continue
		}
		_, err := enode.Parse(enode.ValidSchemes, addr)
		if err == nil {
			enodeString = append(enodeString, addr)
			continue
		}
		_, err = multiAddrFromString(addr)
		if err == nil {
			multiAddrString = append(multiAddrString, addr)
			continue
		}
	}
	return enodeString, multiAddrString
}

func multiAddrFromString(address string) (ma.Multiaddr, error) {
	return ma.NewMultiaddr(address)
}
