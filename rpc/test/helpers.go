package rpctest

import (
	"context"
	"fmt"
	"os"
	"time"

	abciclient "github.com/tendermint/tendermint/abci/client"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/libs/log"
	tmnet "github.com/tendermint/tendermint/libs/net"
	"github.com/tendermint/tendermint/libs/service"
	"github.com/tendermint/tendermint/node"
	"github.com/tendermint/tendermint/rpc/coretypes"
	rpcclient "github.com/tendermint/tendermint/rpc/jsonrpc/client"
)

// Options helps with specifying some parameters for our RPC testing for greater
// control.
type Options struct {
	suppressStdout bool
}

// waitForRPC connects to the RPC service and blocks until a /status call succeeds.
func waitForRPC(ctx context.Context, conf *config.Config) {
	laddr := conf.RPC.ListenAddress
	client, err := rpcclient.New(laddr)
	if err != nil {
		panic(err)
	}
	result := new(coretypes.ResultStatus)
	for {
		_, err := client.Call(ctx, "status", map[string]interface{}{}, result)
		if err == nil {
			return
		}

		fmt.Println("error", err)
		time.Sleep(time.Millisecond)
	}
}

func randPort() int {
	port, err := tmnet.GetFreePort()
	if err != nil {
		panic(err)
	}
	return port
}

// makeAddrs constructs local listener addresses for node services.  This
// implementation uses random ports so test instances can run concurrently.
func makeAddrs() (p2pAddr, rpcAddr string) {
	const addrTemplate = "tcp://127.0.0.1:%d"
	return fmt.Sprintf(addrTemplate, randPort()), fmt.Sprintf(addrTemplate, randPort())
}

func CreateConfig(testName string) (*config.Config, error) {
	c, err := config.ResetTestRoot(testName)
	if err != nil {
		return nil, err
	}

	p2pAddr, rpcAddr := makeAddrs()
	c.P2P.ListenAddress = p2pAddr
	c.RPC.ListenAddress = rpcAddr
	c.Consensus.WalPath = "rpc-test"
	c.RPC.CORSAllowedOrigins = []string{"https://tendermint.com/"}
	return c, nil
}

type ServiceCloser func(context.Context) error

func StartTendermint(ctx context.Context,
	conf *config.Config,
	app abci.Application,
	opts ...func(*Options)) (service.Service, ServiceCloser, error) {

	nodeOpts := &Options{}
	for _, opt := range opts {
		opt(nodeOpts)
	}
	var logger log.Logger
	if nodeOpts.suppressStdout {
		logger = log.NewNopLogger()
	} else {
		logger = log.MustNewDefaultLogger(log.LogFormatPlain, log.LogLevelInfo, false)
	}
	papp := abciclient.NewLocalCreator(app)
	tmNode, err := node.New(conf, logger, papp, nil)
	if err != nil {
		return nil, func(_ context.Context) error { return nil }, err
	}

	err = tmNode.Start()
	if err != nil {
		return nil, func(_ context.Context) error { return nil }, err
	}

	waitForRPC(ctx, conf)

	if !nodeOpts.suppressStdout {
		fmt.Println("Tendermint running!")
	}

	return tmNode, func(ctx context.Context) error {
		if err := tmNode.Stop(); err != nil {
			logger.Error("Error when trying to stop node", "err", err)
		}
		tmNode.Wait()
		os.RemoveAll(conf.RootDir)
		return nil
	}, nil
}

// SuppressStdout is an option that tries to make sure the RPC test Tendermint
// node doesn't log anything to stdout.
func SuppressStdout(o *Options) {
	o.suppressStdout = true
}
