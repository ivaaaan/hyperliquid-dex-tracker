package main

import (
	"bytes"
	"context"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

const erc20StringABI = `[
  {"constant":true,"inputs":[],"name":"name","outputs":[{"name":"","type":"string"}],"stateMutability":"view","type":"function"},
  {"constant":true,"inputs":[],"name":"symbol","outputs":[{"name":"","type":"string"}],"stateMutability":"view","type":"function"},
  {"constant":true,"inputs":[],"name":"decimals","outputs":[{"name":"","type":"uint8"}],"stateMutability":"view","type":"function"}
]`

const erc20Bytes32ABI = `[
  {"constant":true,"inputs":[],"name":"name","outputs":[{"name":"","type":"bytes32"}],"stateMutability":"view","type":"function"},
  {"constant":true,"inputs":[],"name":"symbol","outputs":[{"name":"","type":"bytes32"}],"stateMutability":"view","type":"function"}
]`

type TokenMeta struct {
	Address  common.Address
	Name     string
	Symbol   string
	Decimals uint8
}

func readTokenMeta(ctx context.Context, client *ethclient.Client, addr common.Address) (TokenMeta, error) {
	tm := TokenMeta{Address: addr}

	strABI, _ := abi.JSON(strings.NewReader(erc20StringABI))
	b32ABI, _ := abi.JSON(strings.NewReader(erc20Bytes32ABI))

	{
		data, err := strABI.Pack("decimals")
		if err != nil {
			return tm, err
		}
		out, err := client.CallContract(ctx, ethereum.CallMsg{To: &addr, Data: data}, nil)
		if err == nil && len(out) > 0 {
			var dec *big.Int
			if err := strABI.UnpackIntoInterface(&dec, "decimals", out); err == nil {
				tm.Decimals = uint8(dec.Uint64())
			}
		}
	}

	tm.Symbol = tryReadStringWithBytes32Fallback(ctx, client, addr, &strABI, &b32ABI, "symbol")
	tm.Name = tryReadStringWithBytes32Fallback(ctx, client, addr, &strABI, &b32ABI, "name")

	return tm, nil
}

func tryReadStringWithBytes32Fallback(
	ctx context.Context,
	client *ethclient.Client,
	addr common.Address,
	strABI *abi.ABI,
	b32ABI *abi.ABI,
	method string,
) string {
	if data, err := strABI.Pack(method); err == nil {
		if out, err := client.CallContract(ctx, ethereum.CallMsg{To: &addr, Data: data}, nil); err == nil && len(out) > 0 {
			var s string
			if err := strABI.UnpackIntoInterface(&s, method, out); err == nil && s != "" {
				return s
			}
		}
	}

	if data, err := b32ABI.Pack(method); err == nil {
		if out, err := client.CallContract(ctx, ethereum.CallMsg{To: &addr, Data: data}, nil); err == nil && len(out) > 0 {
			var b [32]byte
			if err := b32ABI.UnpackIntoInterface(&b, method, out); err == nil {
				// trim at first 0x00
				n := bytes.IndexByte(b[:], 0)
				if n == -1 {
					n = len(b)
				}
				return string(b[:n])
			}
		}
	}
	return ""
}
