# DRY for Go
Do not repeat yourself. Re-usable utils for Go apps

## TL;DR

Re-usable Go modules for building [https://safedep.io](https://safedep.io)

## Development

### Setup

* Need `golang-1.24`, refer `.tool-versions`
* Install `gitleaks` following [instructions](https://github.com/gitleaks/gitleaks#installing)
* Install `lefthook`

```bash
go install github.com/evilmartians/lefthook@latest
```

* Install git hooks

```bash
$(go env GOPATH)/bin/lefthook install
```
