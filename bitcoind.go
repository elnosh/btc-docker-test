package btcdocker

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/btcsuite/btcd/rpcclient"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	RPC_USER                     = "testuser"
	RPC_PASSWORD                 = "testpassword"
	BITCOIND_RPC_PORT            = "18443"
	BITCOIND_ZMQPUBRAWBLOCK_PORT = "28334"
	BITCOIND_ZMQPUBRAWTX_PORT    = "28335"
)

type Bitcoind struct {
	testcontainers.Container
	Client *rpcclient.Client

	// ContainerIP is to be used when communicating between containers in the network
	ContainerIP string
	Host        string
	Network     string

	// These are the mapped ports which are exposed to the host
	RpcPort            string
	ZmqpubrawblockPort string
	ZmqpubrawtxPort    string

	Dir string
}

func NewBitcoind(ctx context.Context) (*Bitcoind, error) {
	newNetwork, err := network.New(ctx, network.WithCheckDuplicate())
	if err != nil {
		return nil, fmt.Errorf("error setting up network: %v", err)
	}
	networkName := newNetwork.Name

	currentDir, err := os.Getwd()
	if err != nil {
		return nil, errors.New("error getting current working directory")
	}

	btcdockerDir := filepath.Join(currentDir, "btcdocker")
	if err = os.MkdirAll(btcdockerDir, 0777); err != nil {
		return nil, fmt.Errorf("error creating btcdocker dir: %v", err)
	}

	req := testcontainers.ContainerRequest{
		Image: "polarlightning/bitcoind:28.0",
		ExposedPorts: []string{
			BITCOIND_RPC_PORT,
			BITCOIND_ZMQPUBRAWBLOCK_PORT,
			BITCOIND_ZMQPUBRAWTX_PORT,
		},
		Cmd: []string{
			"bitcoind",
			"-server=1",
			"-regtest=1",
			"-debug=1",
			"-zmqpubrawblock=tcp://0.0.0.0:" + BITCOIND_ZMQPUBRAWBLOCK_PORT,
			"-zmqpubrawtx=tcp://0.0.0.0:" + BITCOIND_ZMQPUBRAWTX_PORT,
			"-rpcbind=0.0.0.0",
			"-rpcallowip=0.0.0.0/0",
			"-rpcport=" + BITCOIND_RPC_PORT,
			"-rpcuser=" + RPC_USER,
			"-rpcpassword=" + RPC_PASSWORD,
			"-txindex=1",
			"-upnp=0",
			"-dnsseed=0",
			"-rest",
			"-listen=1",
			"-listenonion=0",
		},
		Networks: []string{networkName},
		WaitingFor: wait.ForAll(
			wait.ForListeningPort(BITCOIND_RPC_PORT),
			wait.ForListeningPort(BITCOIND_ZMQPUBRAWBLOCK_PORT),
			wait.ForListeningPort(BITCOIND_ZMQPUBRAWTX_PORT),
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

	host, err := container.Host(ctx)
	if err != nil {
		return nil, err
	}

	rpcport, err := container.MappedPort(ctx, BITCOIND_RPC_PORT)
	if err != nil {
		return nil, err
	}

	zmqpubrawblockport, err := container.MappedPort(ctx, BITCOIND_ZMQPUBRAWBLOCK_PORT)
	if err != nil {
		return nil, err
	}

	zmqpubrawtxport, err := container.MappedPort(ctx, BITCOIND_ZMQPUBRAWTX_PORT)
	if err != nil {
		return nil, err
	}

	connConfig := &rpcclient.ConnConfig{
		Host:         host + ":" + rpcport.Port(),
		User:         RPC_USER,
		Pass:         RPC_PASSWORD,
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
		Network:            networkName,
		RpcPort:            rpcport.Port(),
		ZmqpubrawblockPort: zmqpubrawblockport.Port(),
		ZmqpubrawtxPort:    zmqpubrawtxport.Port(),
		Dir:                btcdockerDir,
	}

	return bitcoind, nil
}

func (bitcoind *Bitcoind) Terminate(ctx context.Context) error {
	//delete created dir
	if err := os.RemoveAll(bitcoind.Dir); err != nil {
		return fmt.Errorf("error deleting created btcdocker dir: %v", err)
	}

	if err := bitcoind.Container.Terminate(ctx); err != nil {
		return fmt.Errorf("error terminating container: %v", err)
	}

	return nil
}
