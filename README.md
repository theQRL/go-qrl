[![CircleCI](https://circleci.com/gh/theQRL/go-qrl.svg?style=shield)](https://circleci.com/gh/theQRL/go-qrl)
[![Codacy Badge](https://api.codacy.com/project/badge/Coverage/46d5220277074dc59bb05a15567ec5c5)](https://www.codacy.com/app/qrl/go-qrl?utm_source=github.com&utm_medium=referral&utm_content=theQRL/go-qrl&utm_campaign=Badge_Coverage)
[![Codacy Badge](https://api.codacy.com/project/badge/Grade/46d5220277074dc59bb05a15567ec5c5)](https://www.codacy.com/app/qrl/go-qrl?utm_source=github.com&amp;utm_medium=referral&amp;utm_content=theQRL/go-qrl&amp;utm_campaign=Badge_Grade)
# go-qrl
*WARNING: This is work in progress, NOT FOR PRODUCTION USE*

## Installing

#### Ubuntu

**Dependencies:**

```
sudo apt -y install swig3.0 python3-dev build-essential cmake libhwloc-dev libboost-dev ninja-build pkg-config

```

**Install gcc-5**

```
sudo add-apt-repository ppa:ubuntu-toolchain-r/test
sudo apt-get update
sudo apt-get install gcc-5 g++-5
```



```
go get -d github.com/theQRL/go-qrl
```

This will complain. `package github.com/theQRL/go-qrl: no Go files in $GOPATH/src/github.com/theQRL/go-qrl` Ignore this warning and proceed.

Change into the source directory and build the package

```
cd $GOPATH/src/github.com/theQRL/go-qrl
make build
```


now run `./gqrl` and the node begins syncing.