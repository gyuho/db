#!/usr/bin/env bash
set -e

printf "glide update\n"
glide update --strip-vendor --strip-vcs --update-vendored

printf "glide vc\n"
glide vc --only-code --no-tests

printf "Done!\n"

