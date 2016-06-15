#!/usr/bin/env bash
set -e

glide update --strip-vendor --strip-vcs --update-vendored
glide vc --only-code --no-tests

