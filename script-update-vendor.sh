#!/usr/bin/env bash
set -e

echo "Installing glide..."
GLIDE_ROOT="$GOPATH/src/github.com/Masterminds/glide"
rm -rf $GLIDE_ROOT
go get -v -u github.com/Masterminds/glide
go get -v -u github.com/sgotti/glide-vc
pushd "${GLIDE_ROOT}"
	git reset --hard HEAD
	go install
popd

rm -rf vendor
glide -v
glide update --strip-vendor --strip-vcs --update-vendored
glide vc --only-code --no-tests
