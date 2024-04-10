# bot-banter

[![Build Status](https://github.com/milosgajdos/bot-banter/workflows/CI/badge.svg)](https://github.com/milosgajdos/bot-banter/actions?query=workflow%3ACI)
[![go.dev reference](https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white&style=flat-square)](https://pkg.go.dev/github.com/milosgajdos/bot-banter)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache--2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)


# HOWTO

Run NATS
```
nix-shell -p nats-server
nats-server -js
```

Start a `gobot`:
```
go run ./gobot/...
```

Start a `rustbot`:
```
cargo run --manifest-path rustbot/Cargo.toml
```
