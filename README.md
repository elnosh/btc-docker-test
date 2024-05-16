# BTC Docker Test

Create `Bitcoind` and `Lnd` containers in regtest. Useful for integration testing.

**You need to have Docker installed.**

``` go
import "github.com/elnosh/btc-docker-test"

ctx := context.Background()

bitcoind, err := btcdocker.NewBitcoind(ctx)
if err != nil {
    // handle err
}

blockchainInfo, err := bitcoind.Client.GetBlockchainInfo()

lnd, err := btcdocker.NewLnd(ctx, bitcoind)
if err != nil {
    // handle err
}

req := lnrpc.GetInfoRequest{}
info, err := lnd.Client.GetInfo(ctx, &req)
```

Inspired by [Bitcoind](https://github.com/rust-bitcoin/bitcoind) and [Lnd](https://github.com/bennyhodl/lnd-test-util)