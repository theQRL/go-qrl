GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get

build: deps
	$(GOBUILD) -o gqrl $(GOPATH)/src/github.com/theQRL/go-qrl/cmd/gqrl/main.go
test:
	$(GOTEST) -v ./...
clean:
	$(GOCLEAN)
	rm $(GOPATH)/src/github.com/theQRL/go-qrl/gqrl
deps:
	@if [ -z "$(GOPATH)" ]; then \
	  echo "GOPATH Not Set" && \
	  exit 1; \
	fi
	if [ ! -d "$(GOPATH)/src/github.com/theQRL/qrllib" ]; then \
	  git clone --recurse-submodules https://github.com/cyyber/qrllib $(GOPATH)/src/github.com/theQRL/qrllib && \
	  cmake -DBUILD_GO=ON $(GOPATH)/src/github.com/theQRL/qrllib -B$(GOPATH)/src/github.com/theQRL/qrllib && \
	  make -C $(GOPATH)/src/github.com/theQRL/qrllib; \
	fi
	if [ ! -d "$(GOPATH)/src/github.com/theQRL/qryptonight" ]; then \
	  git clone --recurse-submodules https://github.com/cyyber/qryptonight $(GOPATH)/src/github.com/theQRL/qryptonight && \
	  cmake -DBUILD_GO=ON -DSANITIZE_ADDRESS=OFF -DCMAKE_C_COMPILER=gcc-5 -DCMAKE_CXX_COMPILER=g++-5 $(GOPATH)/src/github.com/theQRL/qryptonight -B$(GOPATH)/src/github.com/theQRL/qryptonight && \
	  make -C $(GOPATH)/src/github.com/theQRL/qryptonight; \
	fi
	$(GOGET) ./...