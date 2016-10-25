#!/usr/bin/env bash
set -e

<<COMMENT
go get -v github.com/golang/protobuf/protoc-gen-go

printf "generating proto in raft/raftpb\n"
protoc --go_out=. raft/raftpb/*.proto

printf "generating proto in raftwal/raftwalpb\n"
protoc --go_out=. raftwal/raftwalpb/*.proto
COMMENT

if ! [[ "$0" =~ "scripts/genproto.sh" ]]; then
	echo "must be run from repository root"
	exit 255
fi

# for now, be conservative about what version of protoc we expect
if ! [[ $(protoc --version) =~ "3.0.0" ]]; then
	echo "could not find protoc 3.0.0, is it installed + in PATH?"
	exit 255
fi

echo "Installing gogo/protobuf..."
GOGOPROTO_ROOT="$GOPATH/src/github.com/gogo/protobuf"
rm -rf $GOGOPROTO_ROOT
go get -v github.com/gogo/protobuf/{proto,protoc-gen-gogo,gogoproto,protoc-gen-gofast}
go get -v golang.org/x/tools/cmd/goimports
pushd "${GOGOPROTO_ROOT}"
	git reset --hard HEAD
	make install
popd

printf "Generating raft/raftpb\n"
protoc --gofast_out=plugins=grpc:. \
	--proto_path=$GOPATH/src:$GOPATH/src/github.com/gogo/protobuf/protobuf:. \
	raft/raftpb/*.proto;

printf "Generating raftwal/raftwalpb\n"
protoc --gofast_out=plugins=grpc:. \
	--proto_path=$GOPATH/src:$GOPATH/src/github.com/gogo/protobuf/protobuf:. \
	raftwal/raftwalpb/*.proto;

printf "Generating raftsnap/raftsnappb\n"
protoc --gofast_out=plugins=grpc:. \
	--proto_path=$GOPATH/src:$GOPATH/src/github.com/gogo/protobuf/protobuf:. \
	raftsnap/raftsnappb/*.proto;
