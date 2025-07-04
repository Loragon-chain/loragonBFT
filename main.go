package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/cometbft/cometbft/privval"
	"github.com/cometbft/cometbft/proxy"
	"golang.org/x/sync/errgroup"

	node "github.com/Loragon-chain/loragonBFT/node"
	cfg "github.com/cometbft/cometbft/config"
	cmtflags "github.com/cometbft/cometbft/libs/cli/flags"
	cmtlog "github.com/cometbft/cometbft/libs/log"
	cmtnode "github.com/cometbft/cometbft/node"

	"github.com/Loragon-chain/loragonBFT/types"
	"github.com/spf13/viper"
)

var homeDir string

func init() {
	flag.StringVar(&homeDir, "cmt-home", "", "Path to the LoragonBFT config directory (if empty, uses $HOME/.loragon)")
}

var (
	logger = cmtlog.NewTMLogger(
		cmtlog.NewSyncWriter(os.Stdout),
	).With("module", "priv_val")
)

func main() {

	flag.Parse()
	if homeDir == "" {
		homeDir = os.ExpandEnv("$HOME/.loragon")
	}

	config := cfg.DefaultConfig()
	config.SetRoot(homeDir)
	viper.SetConfigFile(fmt.Sprintf("%s/%s", homeDir, "config/config.toml"))

	if err := viper.ReadInConfig(); err != nil {
		slog.Error("Reading config: %v", err)
	}
	if err := viper.Unmarshal(config); err != nil {
		slog.Error("Decoding config: %v", err)
	}
	if err := config.ValidateBasic(); err != nil {
		slog.Error("Invalid configuration data: %v", err)
	}
	dbPath := filepath.Join(homeDir, "badger")

	app := NewKVStoreApplication(dbPath)

	pv := privval.LoadFilePV(
		config.PrivValidatorKeyFile(),
		config.PrivValidatorStateFile(),
	)

	nodeKey, err := types.LoadNodeKey(config.NodeKeyFile())
	if err != nil {
		slog.Error("failed to load node's key: %v %v", config.NodeKeyFile(), err)
	}

	logger := cmtlog.NewTMLogger(cmtlog.NewSyncWriter(os.Stdout))
	logger, err = cmtflags.ParseLogLevel(config.LogLevel, logger, cfg.DefaultLogLevel)

	if err != nil {
		slog.Error("failed to parse log level: %v", err)
	}
	ctx, cancelFn := context.WithCancel(context.TODO())
	// config.LogLevel = "debug" // default is info
	node, err := node.NewNode(
		ctx,
		config,
		pv,
		nodeKey,
		proxy.NewLocalClientCreator(app),
		cmtnode.DefaultGenesisDocProviderFunc(config),
		cfg.DefaultDBProvider,
		cmtnode.DefaultMetricsProvider(config.Instrumentation),
		logger,
	)

	if err != nil {
		slog.Error("Creating node: %v", err)
	}

	node.Start()
	defer func() {
		cancelFn()
		// node.Stop()
		// node.Wait()
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
}

// ListenForQuitSignals listens for SIGINT and SIGTERM. When a signal is received,
// the cleanup function is called, indicating the caller can gracefully exit or
// return.
//
// Note, the blocking behavior of this depends on the block argument.
// The caller must ensure the corresponding context derived from the cancelFn is used correctly.
func ListenForQuitSignals(g *errgroup.Group, block bool, cancelFn context.CancelFunc, logger slog.Logger) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	f := func() {
		sig := <-sigCh
		cancelFn()

		logger.Info("caught signal", "signal", sig.String())
	}

	if block {
		g.Go(func() error {
			f()
			return nil
		})
	} else {
		go f()
	}
}
