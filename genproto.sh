#!/usr/bin/env bash
set -e

# for now, be conservative about what version of protoc we expect
if ! [[ $(protoc --version) =~ "3.0.0" ]]; then
	echo "could not find protoc 3.0.0, is it installed + in PATH?"
	exit 255
fi

<<COMMENT
go get -v github.com/golang/protobuf/protoc-gen-go

printf "generating proto in raftpb\n"
protoc --go_out=. raftpb/*.proto

printf "generating proto in walpb\n"
protoc --go_out=. walpb/*.proto
COMMENT

go get -v github.com/gogo/protobuf/proto;
go get -v github.com/gogo/protobuf/protoc-gen-gogo;
go get -v github.com/gogo/protobuf/gogoproto;
go get -v github.com/gogo/protobuf/protoc-gen-gofast;

printf "generating proto in raftpb\n"
protoc --gofast_out=plugins=grpc:. \
	--proto_path=$GOPATH/src:$GOPATH/src/github.com/gogo/protobuf/protobuf:. \
	raftpb/*.proto;

printf "generating proto in walpb\n"
protoc --gofast_out=plugins=grpc:. \
	--proto_path=$GOPATH/src:$GOPATH/src/github.com/gogo/protobuf/protobuf:. \
	walpb/*.proto;
