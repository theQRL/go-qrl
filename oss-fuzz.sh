#!/bin/bash -eu
# Copyright 2022 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
################################################################################

# This sets the -coverpgk for the coverage report when the corpus is executed through go test
coverpkg="github.com/theQRL/go-zond/..."

function coverbuild {
  path=$1
  function=$2
  fuzzer=$3
  tags=""

  if [[ $#  -eq 4 ]]; then
    tags="-tags $4"
  fi
  cd $path
  fuzzed_package=`pwd | rev | cut -d'/' -f 1 | rev`
  cp $GOPATH/ossfuzz_coverage_runner.go ./"${function,,}"_test.go
  sed -i -e 's/FuzzFunction/'$function'/' ./"${function,,}"_test.go
  sed -i -e 's/mypackagebeingfuzzed/'$fuzzed_package'/' ./"${function,,}"_test.go
  sed -i -e 's/TestFuzzCorpus/Test'$function'Corpus/' ./"${function,,}"_test.go

cat << DOG > $OUT/$fuzzer
#/bin/sh

  cd $OUT/$path
  go test -run Test${function}Corpus -v $tags -coverprofile \$1 -coverpkg $coverpkg

DOG

  chmod +x $OUT/$fuzzer
  #echo "Built script $OUT/$fuzzer"
  #cat $OUT/$fuzzer
  cd -
}

function compile_fuzzer() {
  package=$1
  function=$2
  fuzzer=$3
  file=$4

  path=$GOPATH/src/$package

  echo "Building $fuzzer"
  cd $path

  # Install build dependencies
  go mod tidy
  go get github.com/holiman/gofuzz-shim/testing

	if [[ $SANITIZER == *coverage* ]]; then
		coverbuild $path $function $fuzzer
	else
	  gofuzz-shim --func $function --package $package -f $file -o $fuzzer.a
		$CXX $CXXFLAGS $LIB_FUZZING_ENGINE $fuzzer.a -o $OUT/$fuzzer
	fi

  ## Check if there exists a seed corpus file
  corpusfile="${path}/testdata/${fuzzer}_seed_corpus.zip"
  if [ -f $corpusfile ]
  then
    cp $corpusfile $OUT/
    echo "Found seed corpus: $corpusfile"
  fi
  cd -
}

go install github.com/holiman/gofuzz-shim@latest
repo=$GOPATH/src/github.com/theQRL/go-zond

compile_fuzzer github.com/theQRL/go-zond/accounts/abi \
  FuzzABI fuzzAbi \
  $repo/accounts/abi/abifuzzer_test.go

compile_fuzzer github.com/theQRL/go-zond/common/bitutil \
  FuzzEncoder fuzzBitutilEncoder \
  $repo/common/bitutil/compress_test.go

compile_fuzzer github.com/theQRL/go-zond/common/bitutil \
  FuzzDecoder fuzzBitutilDecoder \
  $repo/common/bitutil/compress_test.go

compile_fuzzer github.com/theQRL/go-zond/core/vm/runtime \
  FuzzVmRuntime fuzzVmRuntime\
  $repo/core/vm/runtime/runtime_fuzz_test.go

compile_fuzzer github.com/theQRL/go-zond/core/vm \
  FuzzPrecompiledContracts fuzzPrecompiledContracts\
  $repo/core/vm/contracts_fuzz_test.go,$repo/core/vm/contracts_test.go

compile_fuzzer github.com/theQRL/go-zond/core/types \
  FuzzRLP fuzzRlp \
  $repo/core/types/rlp_fuzzer_test.go

compile_fuzzer github.com/theQRL/go-zond/accounts/keystore \
  FuzzPassword fuzzKeystore \
  $repo/accounts/keystore/keystore_fuzzing_test.go

pkg=$repo/trie/
compile_fuzzer github.com/theQRL/go-zond/trie \
  FuzzTrie fuzzTrie \
  $pkg/trie_test.go,$pkg/database_test.go,$pkg/tracer_test.go,$pkg/proof_test.go,$pkg/iterator_test.go,$pkg/sync_test.go

compile_fuzzer github.com/theQRL/go-zond/trie \
  FuzzStackTrie fuzzStackTrie \
  $pkg/stacktrie_fuzzer_test.go,$pkg/iterator_test.go,$pkg/trie_test.go,$pkg/database_test.go,$pkg/tracer_test.go,$pkg/proof_test.go,$pkg/sync_test.go

#compile_fuzzer tests/fuzzers/snap  FuzzARange fuzz_account_range
compile_fuzzer github.com/theQRL/go-zond/qrl/protocols/snap \
  FuzzARange fuzz_account_range \
  $repo/qrl/protocols/snap/handler_fuzzing_test.go

compile_fuzzer github.com/theQRL/go-zond/qrl/protocols/snap \
  FuzzSRange fuzz_storage_range \
  $repo/qrl/protocols/snap/handler_fuzzing_test.go

compile_fuzzer github.com/theQRL/go-zond/qrl/protocols/snap \
  FuzzByteCodes fuzz_byte_codes \
  $repo/qrl/protocols/snap/handler_fuzzing_test.go

compile_fuzzer github.com/theQRL/go-zond/qrl/protocols/snap \
  FuzzTrieNodes fuzz_trie_nodes\
  $repo/qrl/protocols/snap/handler_fuzzing_test.go

compile_fuzzer github.com/theQRL/go-zond/tests/fuzzers/txfetcher \
  Fuzz fuzzTxfetcher \
  $repo/tests/fuzzers/txfetcher/txfetcher_test.go

compile_fuzzer github.com/theQRL/go-zond/tests/fuzzers/secp256k1 \
  Fuzz fuzzSecp256k1\
  $repo/tests/fuzzers/secp256k1/secp_test.go