# Go bindings to system LLVM

[![Build Status](https://github.com/xgo-dev/llvm/actions/workflows/go.yml/badge.svg)](https://github.com/xgo-dev/llvm/actions/workflows/go.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/xgo-dev/llvm)](https://goreportcard.com/report/github.com/xgo-dev/llvm)
[![GoDoc](https://pkg.go.dev/badge/github.com/xgo-dev/llvm.svg)](https://pkg.go.dev/github.com/xgo-dev/llvm)
<!--
[![GitHub release](https://img.shields.io/github/v/tag/goplus/llvm.svg?label=release)](https://github.com/xgo-dev/llvm/releases)
[![Coverage Status](https://codecov.io/gh/goplus/llvm/branch/main/graph/badge.svg)](https://codecov.io/gh/goplus/llvm)
-->

This library provides bindings to a system-installed LLVM.

Currently supported:

  * LLVM 22 from [apt.llvm.org](http://apt.llvm.org/) on Debian/Ubuntu.
  * LLVM 22 from Homebrew on macOS.
  * A manually built LLVM 22 through the `byollvm` build tag. You need to set
    up `CFLAGS`/`LDFLAGS` yourself in this case.

LLVM 22 is the default and sole supported ABI. Version-selection build tags
are no longer required or supported.

## Usage

If you have a supported LLVM installation, you should be able to do a simple `go get`:

    go get github.com/xgo-dev/llvm

The package links LLVM 22 by default. Use `byollvm` only to supply a custom
LLVM 22 installation.

## License

These LLVM bindings for Go originally come from LLVM, but they have since been [removed](https://discourse.llvm.org/t/rfc-remove-the-go-bindings/65725). Still, they remain under the same license as they were originally, which is the [Apache License 2.0 (with LLVM exceptions)](http://releases.llvm.org/9.0.0/LICENSE.TXT). Check upstream LLVM for detailed copyright information.

This README, the backports\* files, and the Makefile are separate from LLVM but are licensed under the same license.
