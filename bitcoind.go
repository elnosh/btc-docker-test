package btcdocker

import (
	"context"
	"fmt"

	"github.com/btcsuite/btcd/rpcclient"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
)

type BitcoindConfig struct {
	RpcUser     string
	RpcPassword string
}

type Bitcoind struct {
	testcontainers.Container
	Client             *rpcclient.Client
	ContainerIP        string
	Host               string
	RpcPort            string
	ZmqpubrawblockPort string
	ZmqpubrawtxPort    string
	network            string
	config             BitcoindConfig
}

func SetupBitcoind(ctx context.Context, config BitcoindConfig) (*Bitcoind, error) {
	newNetwork, err := network.New(ctx, network.WithCheckDuplicate())
	if err != nil {
		return nil, fmt.Errorf("error setting up network: %v", err)
	}
	networkName := newNetwork.Name

	req := testcontainers.ContainerRequest{
		Image: "polarlightning/bitcoind:26.0",
		ExposedPorts: []string{
			"18443/tcp",
			"18444/tcp",
			"28334/tcp",
			"28335/tcp",
		},
		Cmd: []string{
			"bitcoind",
			"-server=1",
			"-regtest=1",
			"-debug=1",
			"-zmqpubrawblock=tcp://0.0.0.0:28334",
			"-zmqpubrawtx=tcp://0.0.0.0:28335",
			"-zmqpubhashblock=tcp://0.0.0.0:28336",
			"-rpcbind=0.0.0.0",
			"-rpcallowip=0.0.0.0/0",
			"-rpcport=18443",
			"-rpcuser=" + config.RpcUser,
			"-rpcpassword=" + config.RpcPassword,
			"-txindex=1",
			"-upnp=0",
			"-dnsseed=0",
			"-rest",
			"-listen=1",
			"-listenonion=0",
		},
		Networks: []string{networkName},
		WaitingFor: wait.ForAll(
			wait.ForListeningPort("18443/tcp"),
			wait.ForListeningPort("18444/tcp"),
			wait.ForListeningPort("28334/tcp"),
			wait.ForListeningPort("28335/tcp"),
		),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, err
	}

	containerIP, err := container.ContainerIP(ctx)
	if err != nil {
		return nil, err
	}

	rpcport, err := container.MappedPort(ctx, "18443")
	if err != nil {
		return nil, err
	}

	zmqpubrawblockport, err := container.MappedPort(ctx, "28334")
	if err != nil {
		return nil, err
	}

	zmqpubrawtxport, err := container.MappedPort(ctx, "28335")
	if err != nil {
		return nil, err
	}

	host, err := container.Host(ctx)
	if err != nil {
		return nil, err
	}
	connConfig := &rpcclient.ConnConfig{
		Host:         host + ":" + rpcport.Port(),
		User:         config.RpcUser,
		Pass:         config.RpcPassword,
		DisableTLS:   true,
		HTTPPostMode: true,
	}

	rpcClient, err := rpcclient.New(connConfig, nil)
	if err != nil {
		return nil, err
	}

	bitcoind := &Bitcoind{
		Container:          container,
		Client:             rpcClient,
		ContainerIP:        containerIP,
		Host:               host,
		RpcPort:            rpcport.Port(),
		ZmqpubrawblockPort: zmqpubrawblockport.Port(),
		ZmqpubrawtxPort:    zmqpubrawtxport.Port(),
		network:            networkName,
		config:             config,
	}

	return bitcoind, nil
}
