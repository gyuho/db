#!/usr/bin/env bash

<<COMMENT
IGNORE_PKGS="(vendor)"
TESTS=`find . -name \*_test.go | while read a; do dirname $a; done | sort | uniq | egrep -v "$IGNORE_PKGS"`
echo $TESTS

go vet $TESTS

result=$(go tool vet -shadow $TESTS 2>&1 >/dev/null)
echo $result
COMMENT

if ! [[ "$0" =~ "scripts/tests.sh" ]]; then
	echo "must be run from repository root"
	exit 255
fi

IGNORE_PKGS="(vendor|rafttest)"
TESTS=`find . -name \*_test.go | while read a; do dirname $a; done | sort | uniq | egrep -v "$IGNORE_PKGS"`

echo "Checking gofmt..."
fmtRes=$(gofmt -l -s $TESTS 2>&1 >/dev/null)
if [ -n "${fmtRes}" ]; then
	echo -e "gofmt checking failed:\n${fmtRes}"
	exit 255
fi

echo "Checking go vet..."
result=$(go vet $TESTS 2>&1 >/dev/null)
if [ -n "${result}" ]; then
	echo -e "go vet checking failed:\n${result}"
	exit 255
fi

echo "Checking go tool vet -shadow..."
result=$(go tool vet -shadow $TESTS 2>&1 >/dev/null)
if [ -n "${result}" ]; then
	echo -e "go vet shadow checking failed:\n${result}"
	exit 255
fi

echo "Checking gosimple..."
for path in $TESTS; do
	result=`gosimple ${path} || true`
	if [ -n "${result}" ]; then
		echo -e "gosimple checking ${path} failed:\n${result}"
		exit 255
	fi
done

echo "Checking unused..."
for path in $TESTS; do
	result=`unused ${path} || true`
	if [ -n "${result}" ]; then
		echo -e "unused checking ${path} failed:\n${result}"
		exit 255
	fi
done

echo "Running tests...";
go test -v -cover -cpu 1,2,4 $TESTS;
go test -v -cover -cpu 1,2,4 -race $TESTS;

