package node

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"testing"
	"time"

	cmconfig "github.com/cometbft/cometbft/config"
	"github.com/cometbft/cometbft/crypto/ed25519"
	proxy "github.com/cometbft/cometbft/proxy"
	cmtypes "github.com/cometbft/cometbft/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rollkit/rollkit/config"
	"github.com/rollkit/rollkit/da/bitcoin"
	test "github.com/rollkit/rollkit/test/log"
	"github.com/rollkit/rollkit/types"

	"google.golang.org/grpc"

	"google.golang.org/grpc/credentials/insecure"

	goDAproxy "github.com/rollkit/go-da/proxy/grpc"
	goDATest "github.com/rollkit/go-da/test"
)

const (
	// MockDAAddress is the address used by the mock gRPC service
	// NOTE: this should be unique per test package to avoid
	// "bind: listen address already in use" because multiple packages
	// are tested in parallel
	MockDAAddress = "grpc://localhost:7990"

	// MockDANamespace is a sample namespace used by the mock DA client
	MockDANamespace = "00000000000000000000000000000000000000000000000000deadbeef"
	// Bitcoin regnet private keys
	bobPrivateKey      = "5JoQtsKQuH8hC9MyvfJAqo6qmKLm8ePYNucs7tPu2YxG12trzBt"
	internalPrivateKey = "5JGgKfRy6vEcWBpLJV5FXUfMGNXzvdWzQHUM1rVLEUJfvZUSwvS"
)

// TestMain does setup and teardown on the test package
// to make the mock gRPC service available to the nodes
func TestMain(m *testing.M) {
	srv := startMockGRPCServ()
	if srv == nil {
		os.Exit(1)
	}

	btcRegProcess := bitcoin.RegBitcoinProcess{}
	btcRegProcess.RunBitcoinProcess()

	exitCode := m.Run()

	// teardown servers
	srv.GracefulStop()
	btcRegProcess.Stop()

	os.Exit(exitCode)
}

func startMockGRPCServ() *grpc.Server {
	srv := goDAproxy.NewServer(goDATest.NewDummyDA(), grpc.Creds(insecure.NewCredentials()))
	addr, _ := url.Parse(MockDAAddress)
	lis, err := net.Listen("tcp", addr.Host)
	if err != nil {
		panic(err)
	}
	go func() {
		_ = srv.Serve(lis)
	}()
	return srv
}

type NodeType int

const (
	Full NodeType = iota
	Light
)

// startNode starts the full node and stops it when the test is done
func startNodeWithCleanup(t *testing.T, node Node) {
	require.False(t, node.IsRunning())
	require.NoError(t, node.Start())
	require.True(t, node.IsRunning())
	t.Cleanup(func() {
		cleanUpNode(node, t)
	})
}

// cleanUpNode stops the node and checks if it is running
func cleanUpNode(node Node, t *testing.T) {
	assert.NoError(t, node.Stop())
	assert.False(t, node.IsRunning())
}

// initializeAndStartNode initializes and starts a node of the specified type.
func initAndStartNodeWithCleanup(ctx context.Context, t *testing.T, nodeType NodeType, aggregatorMode bool) Node {
	node, _ := setupTestNode(ctx, t, nodeType, aggregatorMode, "", nil, nil)
	startNodeWithCleanup(t, node)

	return node
}

// setupTestNode sets up a test node based on the NodeType.
func setupTestNode(ctx context.Context, t *testing.T, nodeType NodeType, aggregatorMode bool, chainId string, genesis *cmtypes.GenesisDoc, privateKey ed25519.PrivKey) (Node, ed25519.PrivKey) {
	node, privKey, err := newTestNode(ctx, t, nodeType, aggregatorMode, chainId, genesis, privateKey)
	require.NoError(t, err)
	require.NotNil(t, node)

	return node, privKey
}

// newTestNode creates a new test node based on the NodeType.
// custom genesis doc
func newTestNode(ctx context.Context, t *testing.T, nodeType NodeType, aggregatorMode bool, chainId string, genesis *cmtypes.GenesisDoc, genesisValidatorKey ed25519.PrivKey) (Node, ed25519.PrivKey, error) {
	config := config.NodeConfig{
		DAAddress:   MockDAAddress,
		DANamespace: MockDANamespace,
		BitcoinManagerConfig: config.BitcoinManagerConfig{
			BtcSignerPriv:         bobPrivateKey,
			BtcSignerInternalPriv: internalPrivateKey,
			BtcHost:               "localhost:18443",
			BtcUser:               "regtest",
			BtcPass:               "regtest",
			BtcHTTPPostMode:       true,
			BtcDisableTLS:         true,
			BtcBlockTime:          3 * time.Second,
		},
		Aggregator: aggregatorMode,
	}

	switch nodeType {
	case Light:
		config.Light = true
	case Full:
		config.Light = false
	default:
		panic(fmt.Sprintf("invalid node type: %v", nodeType))
	}
	app := setupMockApplication()
	if genesis == nil && genesisValidatorKey == nil {
		genesis, genesisValidatorKey = types.GetGenesisWithPrivkey(chainId)
	}
	signingKey, err := types.PrivKeyToSigningKey(genesisValidatorKey)
	if err != nil {
		return nil, nil, err
	}

	key := generateSingleKey()

	logger := test.NewFileLogger(t)
	node, err := NewNode(ctx, config, key, signingKey, proxy.NewLocalClientCreator(app), genesis, DefaultMetricsProvider(cmconfig.DefaultInstrumentationConfig()), logger)
	return node, genesisValidatorKey, err
}

func TestNewNode(t *testing.T) {
	ctx := context.Background()

	ln := initAndStartNodeWithCleanup(ctx, t, Light, false)
	require.IsType(t, new(LightNode), ln)
	fn := initAndStartNodeWithCleanup(ctx, t, Full, false)
	require.IsType(t, new(FullNode), fn)
}
