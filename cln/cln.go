package cln

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	btcdocker "github.com/elnosh/btc-docker-test"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	CLN_REST_PORT = "8080"
	CLN_P2P_PORT  = "9735"
)

type CLN struct {
	testcontainers.Container

	// ContainerIP is to be used when communicating between containers in the network
	ContainerIP string
	Host        string
	Network     string

	// These are the mapped ports which are exposed to the host
	RestPort string
	P2PPort  string

	CLNDir string
	Rune   string
}

func NewCLN(ctx context.Context, bitcoind *btcdocker.Bitcoind) (*CLN, error) {
	randomId := strconv.Itoa(rand.Int())

	clnDir := filepath.Join(bitcoind.Dir, randomId)
	if err := os.MkdirAll(clnDir, 0777); err != nil {
		return nil, fmt.Errorf("error creating cln dir: %v", err)
	}

	clnReq := testcontainers.ContainerRequest{
		Image: "polarlightning/clightning:24.11.1",
		ExposedPorts: []string{
			CLN_REST_PORT,
			CLN_P2P_PORT,
		},
		Networks: []string{bitcoind.Network},
		Cmd: []string{
			"lightningd",
			"--addr=0.0.0.0:9735",
			"--network=regtest",
			"--bitcoin-rpcuser=" + btcdocker.RPC_USER,
			"--bitcoin-rpcpassword=" + btcdocker.RPC_PASSWORD,
			"--bitcoin-rpcconnect=" + bitcoind.ContainerIP,
			"--bitcoin-rpcport=18443",
			"--log-level=debug",
			"--dev-bitcoind-poll=2",
			"--dev-fast-gossip",
			"--grpc-port=11001",
			"--log-file=-",
			"--log-file=/home/clightning/.lightning/debug.log",
			"--clnrest-port=8080",
			"--clnrest-protocol=http",
			"--clnrest-host=0.0.0.0",
			"--developer",
		},
		HostConfigModifier: func(hc *container.HostConfig) {
			hc.Mounts = []mount.Mount{
				{
					Type:   mount.Type("bind"),
					Source: clnDir,
					Target: "/home/clightning/.lightning",
					BindOptions: &mount.BindOptions{
						CreateMountpoint: true,
					},
				},
			}
		},
		WaitingFor: wait.ForAll(
			wait.ForListeningPort(CLN_REST_PORT),
			wait.ForListeningPort(CLN_P2P_PORT),
		),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: clnReq,
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

	restPort, err := container.MappedPort(ctx, CLN_REST_PORT)
	if err != nil {
		return nil, err
	}

	p2pPort, err := container.MappedPort(ctx, CLN_P2P_PORT)
	if err != nil {
		return nil, err
	}

	_, reader, err := container.Exec(ctx, []string{
		"lightning-cli",
		"--rpc-file", "/home/clightning/.lightning/regtest/lightning-rpc",
		"--regtest",
		"createrune",
	})
	if err != nil {
		return nil, err
	}

	response, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	var runeResponse struct {
		Rune string `json:"rune"`
	}

	start := strings.IndexByte(string(response), '{')
	if err := json.Unmarshal(response[start:], &runeResponse); err != nil {
		return nil, err
	}

	cln := &CLN{
		Container:   container,
		ContainerIP: containerIP,
		Host:        host,
		Network:     bitcoind.Network,
		RestPort:    restPort.Port(),
		P2PPort:     p2pPort.Port(),
		CLNDir:      clnDir,
		Rune:        runeResponse.Rune,
	}

	return cln, nil
}
