#!/usr/bin/env bash
set -e

go test -v -run=xxx -benchmem -bench=Benchmark
