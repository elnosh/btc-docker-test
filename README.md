# BTC Docker Test

Utility to create `bitcoind` and other containers in regtest. Useful for integration testing.

### Supports

- [x] bitcoind
- [x] LND
- [x] CLN


**You need to have Docker installed.**

``` go
import (
    btcdocker "github.com/elnosh/btc-docker-test"
    "github.com/elnosh/btc-docker-test/lnd"
    "github.com/elnosh/btc-docker-test/cln"
) 

ctx := context.Background()

// create bitcoind container
bitcoind, err := btcdocker.NewBitcoind(ctx)
if err != nil {
    // handle err
}

blockchainInfo, err := bitcoind.Client.GetBlockchainInfo()

// create LND container
lnd, err := lnd.NewLnd(ctx, bitcoind)
if err != nil {
    // handle err
}

req := lnrpc.GetInfoRequest{}
info, err := lnd.Client.GetInfo(ctx, &req)

// create CLN container
cln, err := cln.NewCLN(ctx, bitcoind)
if err != nil {
    // handle err
}
```

Inspired by [Bitcoind](https://github.com/rust-bitcoin/bitcoind) and [Lnd](https://github.com/bennyhodl/lnd-test-util)
