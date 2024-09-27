package btcdocker

import (
	"context"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strconv"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lnrpc/invoicesrpc"
	"github.com/lightningnetwork/lnd/macaroons"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"gopkg.in/macaroon.v2"
)

const (
	LND_GRPC_PORT = "10009"
	LND_REST_PORT = "8080"
	LND_P2P_PORT  = "9735"
)

type Lnd struct {
	testcontainers.Container
	Client         lnrpc.LightningClient
	InvoicesClient invoicesrpc.InvoicesClient

	// ContainerIP is to be used when communicating between containers in the network
	ContainerIP string
	Host        string

	// These are the mapped ports which are exposed to the host
	GrpcPort string
	RestPort string
	P2PPort  string

	LndDir        string
	AdminMacaroon []byte
}

func NewLnd(ctx context.Context, bitcoind *Bitcoind) (*Lnd, error) {
	randomId := strconv.Itoa(rand.Int())

	lndDir := filepath.Join(bitcoind.dir, randomId)
	if err := os.MkdirAll(lndDir, 0777); err != nil {
		return nil, fmt.Errorf("error creating lnd dir: %v", err)
	}

	rpchost := bitcoind.ContainerIP + ":" + BITCOIND_RPC_PORT
	lndReq := testcontainers.ContainerRequest{
		Image: "polarlightning/lnd:0.17.4-beta",
		ExposedPorts: []string{
			LND_REST_PORT,
			LND_P2P_PORT,
			LND_GRPC_PORT,
		},
		Networks: []string{bitcoind.network},
		Cmd: []string{
			"lnd",
			"--noseedbackup",
			"--debuglevel=debug",
			"--listen=0.0.0.0:" + LND_P2P_PORT,
			"--rpclisten=0.0.0.0:" + LND_GRPC_PORT,
			"--restlisten=0.0.0.0:" + LND_REST_PORT,
			"--protocol.wumbo-channels",
			"--bitcoin.active",
			"--bitcoin.regtest",
			"--bitcoin.node=bitcoind",
			"--bitcoind.rpchost=" + rpchost,
			"--bitcoind.rpcuser=" + RPC_USER,
			"--bitcoind.rpcpass=" + RPC_PASSWORD,
			"--bitcoind.zmqpubrawblock=tcp://" + bitcoind.ContainerIP + ":" + BITCOIND_ZMQPUBRAWBLOCK_PORT,
			"--bitcoind.zmqpubrawtx=tcp://" + bitcoind.ContainerIP + ":" + BITCOIND_ZMQPUBRAWTX_PORT,
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
			wait.ForListeningPort(LND_REST_PORT),
			wait.ForListeningPort(LND_P2P_PORT),
			wait.ForListeningPort(LND_GRPC_PORT),
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

	grpcPort, err := container.MappedPort(ctx, LND_GRPC_PORT)
	if err != nil {
		return nil, err
	}

	restPort, err := container.MappedPort(ctx, LND_REST_PORT)
	if err != nil {
		return nil, err
	}

	p2pPort, err := container.MappedPort(ctx, LND_P2P_PORT)
	if err != nil {
		return nil, err
	}

	macaroonFile, err := container.CopyFileFromContainer(ctx, "/home/lnd/.lnd/data/chain/bitcoin/regtest/admin.macaroon")
	if err != nil {
		return nil, fmt.Errorf("error getting macaroon file from container: %v", err)
	}

	macaroonBytes, err := io.ReadAll(macaroonFile)
	if err != nil {
		return nil, fmt.Errorf("error reading macaroon file: %v", err)
	}

	lndHost := host + ":" + grpcPort.Port()
	tlsCert := filepath.Join(lndDir, "tls.cert")
	lightningClient, err := setupLndClient(lndHost, tlsCert, macaroonBytes)
	if err != nil {
		return nil, fmt.Errorf("error setting up lightning client: %v", err)
	}

	invoicesClient, err := setupInvoicesClient(lndHost, tlsCert, macaroonBytes)
	if err != nil {
		return nil, fmt.Errorf("error setting up invoices client: %v", err)
	}

	lnd := &Lnd{
		Container:      container,
		Client:         lightningClient,
		InvoicesClient: invoicesClient,
		ContainerIP:    containerIP,
		Host:           host,
		GrpcPort:       grpcPort.Port(),
		RestPort:       restPort.Port(),
		P2PPort:        p2pPort.Port(),
		LndDir:         lndDir,
		AdminMacaroon:  macaroonBytes,
	}

	return lnd, nil
}

func getClientConn(host, tlsCert string, macaroonBytes []byte) (grpc.ClientConnInterface, error) {
	creds, err := credentials.NewClientTLSFromFile(tlsCert, "")
	if err != nil {
		return nil, fmt.Errorf("error setting tls creds: %v", err)
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

	return conn, nil
}

func setupLndClient(host string, tlsCert string, macaroonBytes []byte) (lnrpc.LightningClient, error) {
	conn, err := getClientConn(host, tlsCert, macaroonBytes)
	if err != nil {
		return nil, fmt.Errorf("error getting client connection: %v", err)
	}
	grpcClient := lnrpc.NewLightningClient(conn)
	return grpcClient, nil
}

func setupInvoicesClient(host string, tlsCert string, macaroonBytes []byte) (invoicesrpc.InvoicesClient, error) {
	conn, err := getClientConn(host, tlsCert, macaroonBytes)
	if err != nil {
		return nil, fmt.Errorf("error getting client connection: %v", err)
	}
	invoicesClient := invoicesrpc.NewInvoicesClient(conn)
	return invoicesClient, nil
}
