package btcdocker

import (
	"context"
	"fmt"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

type BitcoindConfig struct {
	RpcUser     string
	RpcPassword string
}

type BitcoindContainer struct {
	testcontainers.Container
	Host string
}

func SetupBitcoindContainer(ctx context.Context, config BitcoindConfig) (*BitcoindContainer, error) {
	req := testcontainers.ContainerRequest{
		Image:        "ghcr.io/sethforprivacy/bitcoind:26.1",
		ExposedPorts: []string{"8332/tcp"},
		Env: map[string]string{
			"REGTEST":     "1",
			"RPCUSER":     config.RpcUser,
			"RPCPASSWORD": config.RpcPassword,
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

	ip, err := container.Host(ctx)
	if err != nil {
		return nil, err
	}

	mappedPort, err := container.MappedPort(ctx, "8332")
	if err != nil {
		return nil, err
	}

	host := fmt.Sprintf("%s:%s", ip, mappedPort.Port())
	return &BitcoindContainer{Container: container, Host: host}, nil
}
