# bot-banter

[![Build Status](https://github.com/milosgajdos/bot-banter/workflows/CI/badge.svg)](https://github.com/milosgajdos/bot-banter/actions?query=workflow%3ACI)
[![go.dev reference](https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white&style=flat-square)](https://pkg.go.dev/github.com/milosgajdos/bot-banter)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache--2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)


# HOWTO

There are a few prerequisites:
* [nats](https://nats.io/)
* [ollama](https://ollama.com/)
* sound/audio libraries on some platforms

## Run NATS

Both bots use [nats](https://nats.io/) as their communcation channel.

### Homebrew

Install
```shell
brew tap nats-io/nats-tools
brew install nats nats-server
```

Run:
```shell
nats-server -js
```

### Nix

```shell
nix-shell -p nats-server natscli
nats-server -js
```

## Run Ollama

Download it from the [official site](https://ollama.com/) or see for the Nix install below.

### Nix

```
nix-shell -p ollama
```

Run a model you decide to use
```shell
ollama run llama2
```
## Audio libraries

If you are running on Linux you need to install the following libraries -- assuming you want to play with the bot speaking service

> [!NOTE]
> This is for Ubuntu Linux, other distros have likely different package names
```shell
sudo apt install -y --no-install-recommends libasound2-dev pkg-config
```

## Run the bots

Start the `gobot`:
```shell
go run ./gobot/...
```

Start the `rustbot`:
```shell
cargo run --manifest-path rustbot/Cargo.toml
```
