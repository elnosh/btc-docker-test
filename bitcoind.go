package btcdocker

import (
	"context"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

type BitcoindConfig struct {
	RpcUser     string
	RpcPassword string
}

type BitcoindContainer struct {
	testcontainers.Container
	Host               string
	RpcPort            string
	ZmqpubrawblockPort string
	ZmqpubrawtxPort    string
}

func SetupBitcoindContainer(ctx context.Context, config BitcoindConfig) (*BitcoindContainer, error) {
	bitcoinversion := "26.1"
	req := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context: "./docker/bitcoind",
			BuildArgs: map[string]*string{
				"BITCOIN_VERSION": &bitcoinversion,
			},
		},
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
			"-zmqpubrawblock=tcp://0.0.0.0:28334",
			"-zmqpubrawtx=tcp://0.0.0.0:28335",
			"-zmqpubhashblock=tcp://0.0.0.0:28336",
			"-rpcbind=0.0.0.0",
			"-rpcallowip=0.0.0.0/0",
			"-rpcport=18443",
			"-rpcuser=" + config.RpcUser,
			"-rpcpassword=" + config.RpcPassword,
			"-txindex=1",
			"-dnsseed=0",
		},
		WaitingFor: wait.ForExposedPort(),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, err
	}

	host, err := container.Host(ctx)
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

	return &BitcoindContainer{Container: container, Host: host,
		RpcPort: rpcport.Port(), ZmqpubrawblockPort: zmqpubrawblockport.Port(),
		ZmqpubrawtxPort: zmqpubrawtxport.Port()}, nil
}
