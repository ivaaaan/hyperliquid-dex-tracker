package evm

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

type UniswapV3DEX struct {
	name       string
	address    string
	startBlock uint64
	client     *ethclient.Client
}

func NewUniswapV3DEX(
	name string,
	address string,
	client *ethclient.Client,
	options ...func(*UniswapV3DEX),
) *UniswapV3DEX {
	dex := &UniswapV3DEX{
		name:    name,
		address: address,
		client:  client,
	}
	for _, option := range options {
		option(dex)
	}

	return dex
}

func WithStartBlock(startBlock uint64) func(*UniswapV3DEX) {
	return func(u *UniswapV3DEX) {
		u.startBlock = startBlock
	}
}

func (u *UniswapV3DEX) Name() string {
	return u.name
}

func (u *UniswapV3DEX) StreamEvents(ctx context.Context) (<-chan Result, error) {
	events := make(chan Result)

	contractAbi, err := abi.JSON(strings.NewReader(u.abi()))
	if err != nil {
		return nil, fmt.Errorf("failed to parse contract ABI: %w", err)
	}

	go func() {
		defer close(events)

		header, err := u.client.HeaderByNumber(ctx, nil)
		if err != nil {
			events <- Result{Err: fmt.Errorf("failed to get latest block: %w", err)}
			return
		}

		const maxRange uint64 = 500
		const lookback uint64 = 500

		startBlock := u.startBlock
		if startBlock == 0 {
			latest := header.Number.Uint64()
			if latest > lookback {
				startBlock = latest - lookback + 1
			} else {
				startBlock = 0
			}
		}

		for {
			header, err := u.client.HeaderByNumber(ctx, nil)
			if err != nil {
				events <- Result{Err: fmt.Errorf("failed to refresh latest block: %w", err)}
				// small pause, then retry unless ctx canceled
				select {
				case <-ctx.Done():
					return
				case <-time.After(1 * time.Second):
					continue
				}
			}

			latest := header.Number.Uint64()
			if startBlock > latest {
				// nothing new yet; wait and try again
				select {
				case <-ctx.Done():
					return
				case <-time.After(1 * time.Second):
					continue
				}
			}

			toBlock := min(startBlock+maxRange-1, latest)

			query := ethereum.FilterQuery{
				Addresses: []common.Address{common.HexToAddress(u.address)},
				FromBlock: big.NewInt(int64(startBlock)),
				ToBlock:   big.NewInt(int64(toBlock)),
			}

			log.Printf("fetching logs for %s from block %d to %d", u.name, startBlock, toBlock)

			logs, err := u.client.FilterLogs(ctx, query)
			if err != nil {
				events <- Result{Err: err}
			}

			for _, vLog := range logs {
				var event PoolCreated

				err := contractAbi.UnpackIntoInterface(&event, "PoolCreated", vLog.Data)
				if err != nil {
					// e.g not supported event
					continue
				}

				var topics [4]string
				for i := range vLog.Topics {
					topics[i] = vLog.Topics[i].Hex()
				}

				events <- Result{Event: &PoolCreatedEvent{
					PoolAddress: event.Pool.Hex(),
					TokenA:      topics[1],
					TokenB:      topics[2],
					DexName:     u.name,
				}}

			}

			startBlock = toBlock + 1
			if toBlock == latest {
				select {
				case <-ctx.Done():
					return
				case <-time.After(5 * time.Second):
				}
			}

		}
	}()

	return events, nil
}

func (u *UniswapV3DEX) abi() string {
	return `[{
    "anonymous": false,
    "inputs": [
      {"indexed": true,  "internalType": "address", "name": "token0",      "type": "address"},
      {"indexed": true,  "internalType": "address", "name": "token1",      "type": "address"},
      {"indexed": true,  "internalType": "uint24",  "name": "fee",         "type": "uint24"},
      {"indexed": false, "internalType": "int24",   "name": "tickSpacing", "type": "int24"},
      {"indexed": false, "internalType": "address", "name": "pool",        "type": "address"}
    ],
    "name": "PoolCreated",
    "type": "event"
  }]`
}
