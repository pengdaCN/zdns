#!/usr/bin/env bash

code_dir=$(basename `pwd`)
cd ..

dir=$(mktemp -d)
cp $code_dir/* $dir -r
cd $dir
rm .git -rf
rm .idea -rf
rm go.mod go.sum -f
rm install-pro.sh
rm Makefile
rm pkg/puredns/t-dns -rf
#rm pkg/zdns/{lookup.go,ulimit_check.go,ulimit_check_unknown.go,zdns.go} -f

find -iname '*.go' -exec sed -i 's@github.com/zmap/zdns@npd/pkg/xzdns@' {} \;
find -iname '*.go' -exec sed -i 's@npd/pkg/xzdns/pkg/@npd/pkg/xzdns/@' {} \;

cd ..
dst=~/pro_code/npd/pkg/xzdns
mkdir -p $dst
cp $dir/pkg/* $dir/internal $dir/cachehash $dir/iohandlers $dst -r

rm $dir -rf