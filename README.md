# HypeEVM DEX Pools Tracker

Simple Telegram bot to track new liquidity pools on HypeEVM-based DEXes. It works by monitoring factory contracts for new pair creation events and sends notifications to a specified Telegram chat.

## Running 

Simply clone the repository and configure `.env` file with your settings. 

## Adding new DEXes

Right now only UniswapV3-like DEXes are supported. To add a new DEX, you need to provide the factory contract address and the event signature for pair creation in the `main.go`. 

Dynamic addition of DEXes via configuration files is not supported yet. You can also implement an interface to support other types of DEXes. Check the `dex` package for more details.
