package btcdocker

import (
	"context"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strconv"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/macaroons"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"gopkg.in/macaroon.v2"
)

type Lnd struct {
	testcontainers.Container
	Client lnrpc.LightningClient
	// ContainerIP is to be used when communicating between containers in the network
	ContainerIP string
	Host        string
	GrpcPort    string
	RestPort    string
	P2PPort     string
	LndDir      string
}

func NewLnd(ctx context.Context, bitcoind *Bitcoind) (*Lnd, error) {
	randomId := strconv.Itoa(rand.Int())
	lndDir := filepath.Join(bitcoind.dir, randomId)

	rpchost := bitcoind.ContainerIP + ":18443"
	lndReq := testcontainers.ContainerRequest{
		Image: "polarlightning/lnd:0.17.4-beta",
		ExposedPorts: []string{
			"8080/tcp",
			"9735/tcp",
			"10009/tcp",
		},
		Networks: []string{bitcoind.network},
		Cmd: []string{
			"lnd",
			"--noseedbackup",
			"--listen=0.0.0.0:9735",
			"--rpclisten=0.0.0.0:10009",
			"--restlisten=0.0.0.0:8080",
			"--bitcoin.active",
			"--bitcoin.regtest",
			"--bitcoin.node=bitcoind",
			"--bitcoind.rpchost=" + rpchost,
			"--bitcoind.rpcuser=" + RPC_USER,
			"--bitcoind.rpcpass=" + RPC_PASSWORD,
			"--bitcoind.zmqpubrawblock=tcp://" + bitcoind.ContainerIP + ":28334",
			"--bitcoind.zmqpubrawtx=tcp://" + bitcoind.ContainerIP + ":28335",
		},
		HostConfigModifier: func(hc *container.HostConfig) {
			hc.Mounts = []mount.Mount{
				{
					Type:   mount.Type("bind"),
					Source: lndDir,
					Target: "/home/lnd/.lnd",
					BindOptions: &mount.BindOptions{
						CreateMountpoint: true,
					},
				},
			}
		},
		WaitingFor: wait.ForAll(
			wait.ForListeningPort("8080/tcp"),
			wait.ForListeningPort("9735/tcp"),
			wait.ForListeningPort("10009/tcp"),
		),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: lndReq,
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

	grpcPort, err := container.MappedPort(ctx, "10009")
	if err != nil {
		return nil, err
	}

	restPort, err := container.MappedPort(ctx, "8080")
	if err != nil {
		return nil, err
	}

	p2pPort, err := container.MappedPort(ctx, "9735")
	if err != nil {
		return nil, err
	}

	lndHost := host + ":" + grpcPort.Port()
	lndClient, err := SetupLndClient(lndHost, lndDir)
	if err != nil {
		return nil, fmt.Errorf("error setting up lnd client: %v", err)
	}

	lnd := &Lnd{
		Container:   container,
		Client:      lndClient,
		ContainerIP: containerIP,
		Host:        host,
		GrpcPort:    grpcPort.Port(),
		RestPort:    restPort.Port(),
		P2PPort:     p2pPort.Port(),
		LndDir:      lndDir,
	}

	return lnd, nil
}

func SetupLndClient(host string, lndDir string) (lnrpc.LightningClient, error) {
	tlsCert := filepath.Join(lndDir, "/tls.cert")
	creds, err := credentials.NewClientTLSFromFile(tlsCert, "")
	if err != nil {
		return nil, fmt.Errorf("error setting tls creds: %v", err)
	}

	macaroonFile := filepath.Join(lndDir, "/data/chain/bitcoin/regtest/admin.macaroon")
	macaroonBytes, err := os.ReadFile(macaroonFile)
	if err != nil {
		return nil, fmt.Errorf("error reading macaroon: %v", err)
	}

	macaroon := &macaroon.Macaroon{}
	if err = macaroon.UnmarshalBinary(macaroonBytes); err != nil {
		return nil, fmt.Errorf("error unmarshalling macaroon: %v", err)
	}

	macarooncreds, err := macaroons.NewMacaroonCredential(macaroon)
	if err != nil {
		return nil, fmt.Errorf("error setting macaroon creds: %v", err)
	}

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(creds),
		grpc.WithPerRPCCredentials(macarooncreds),
	}

	conn, err := grpc.Dial(host, opts...)
	if err != nil {
		return nil, fmt.Errorf("error setting up grpc client: %v", err)
	}

	grpcClient := lnrpc.NewLightningClient(conn)
	return grpcClient, nil
}
