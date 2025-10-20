package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/ivaaaan/hyperliquid-dex-monitor/evm"
	"github.com/joho/godotenv"
	"golang.org/x/sync/errgroup"
)

const rpcURL = "https://rpc.hyperliquid.xyz/evm"

type DexMeta struct {
	Name       string
	URL        string
	TradeURLFn func(tokenA, tokenB string) string
}

var Dexes = map[string]DexMeta{
	"prjx": {Name: "Prjx", URL: "https://prjx.com", TradeURLFn: func(tokenA, tokenB string) string {
		return fmt.Sprintf("https://prjx.com/swap?inputCurrency=%s&outputCurrency=%s", tokenA, tokenB)
	},
	},

	"hyperswap": {Name: "Hyperswap", URL: "https://hyperswap.com", TradeURLFn: func(tokenA, tokenB string) string {
		return fmt.Sprintf("https://app.hyperswap.exchange/#/swap?inputCurrency=%s&outputCurrency=%s", tokenA, tokenB)
	},
	},
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("error loading .env file")
	}

	bot, err := tgbotapi.NewBotAPI(os.Getenv("TELEGRAM_BOT_TOKEN"))
	if err != nil {
		log.Fatalf("create telegram bot: %v", err)
	}

	chatID, err := strconv.ParseInt(os.Getenv("TELEGRAM_CHAT_ID"), 10, 64)
	if err != nil {
		log.Fatalf("invalid TELEGRAM_CHAT_ID: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	client, err := ethclient.DialContext(ctx, rpcURL)
	if err != nil {
		log.Fatalf("connect to RPC: %v", err)
	}

	defer client.Close()

	prjx := evm.NewUniswapV3DEX("prjx", "0xFf7B3e8C00e57ea31477c32A5B52a58Eea47b072", client)
	hyperswap := evm.NewUniswapV3DEX("hyperswap", "0xB1c0fa0B789320044A6F623cFe5eBda9562602E3", client)

	dexes := []evm.DEX{prjx, hyperswap}

	events := make(chan evm.Result)

	eg, ctx := errgroup.WithContext(ctx)

	for _, dex := range dexes {
		eg.Go(func() error {
			dexevents, err := dex.StreamEvents(ctx)
			if err != nil {
				return fmt.Errorf("stream events: %w", err)
			}

			for {
				select {
				case <-ctx.Done():
					return nil
				case event, ok := <-dexevents:
					if !ok {
						return nil
					}
					events <- event
				}
			}
		})
	}

	eg.Go(func() error {
		for {
			select {
			case <-ctx.Done():
				return nil
			case res := <-events:
				if res.Err != nil {
					log.Printf("error: %v", res.Err)
					continue
				}

				poolCreated, ok := res.Event.(*evm.PoolCreatedEvent)
				if !ok {
					log.Printf("unknown event type: %T", res.Event)
					continue
				}

				token0, err := readTokenMeta(ctx, client, common.HexToAddress(poolCreated.TokenA))
				if err != nil {
					log.Printf("failed to read token0 meta: %v", err)
					continue
				}

				token1, err := readTokenMeta(ctx, client, common.HexToAddress(poolCreated.TokenB))
				if err != nil {
					log.Printf("failed to read token1 meta: %v", err)
					continue
				}

				msg := fmt.Sprintf("New Pool Created %s/%s. Swap URL: %s", token0.Symbol, token1.Symbol, Dexes[poolCreated.DexName].TradeURLFn(token0.Address.Hex(), token1.Address.Hex()))
				if _, err := bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf(msg))); err != nil {
					log.Printf("failed to send telegram message: %v", err)
				}

			}
		}
	})

	if err := eg.Wait(); err != nil {
		log.Fatalf("error: %v", err)
	}
}
