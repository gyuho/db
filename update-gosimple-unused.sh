#!/usr/bin/env bash

echo "Installing gosimple, unused..."
DOMINIK_ROOT="$GOPATH/src/honnef.co/go"
rm -rf $DOMINIK_ROOT
mkdir -p $DOMINIK_ROOT
cd $DOMINIK_ROOT
git clone git@github.com:dominikh/go-simple.git $DOMINIK_ROOT/simple
git clone git@github.com:dominikh/go-unused.git $DOMINIK_ROOT/unused
cd $DOMINIK_ROOT/simple && go get -d ./...
cd $DOMINIK_ROOT/simple/cmd/gosimple && go install
cd $DOMINIK_ROOT/unused && go get -d ./...
cd $DOMINIK_ROOT/unused/cmd/unused && go install

