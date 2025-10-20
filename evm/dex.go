package evm

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

const (
	POOL_CREATED = "POOL_CREATED"
)

type Event interface {
	GetType() string
}

type PoolCreatedEvent struct {
	PoolAddress string
	TokenA      string
	TokenB      string
	DexName     string
}

type PoolCreated struct {
	TickSpacing *big.Int       `abi:"tickSpacing"`
	Pool        common.Address `abi:"pool"`
}

func (p PoolCreatedEvent) GetType() string {
	return POOL_CREATED
}

type Result struct {
	Event Event
	Err   error
}

type DEX interface {
	StreamEvents(context.Context) (<-chan Result, error)
	Name() string
}
