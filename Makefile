BIN=bin
TARGET=target
LIBSRC=$(wildcard **/*.go) $(wildcard **/**/*.go) $(wildcard **/**/**/*.go)
LIBRPC=$(wildcard rpc/*.proto)
EXECSRC=$(wildcard cmd/**/*.go)
EXECDIRS=$(sort $(dir $(EXECSRC)))
EXEC=$(patsubst cmd/%/,$(BIN)/%,$(EXECDIRS))
COVERPROF=test.coverprofile
PROTO=rpc/*.pb.go
GOMOCKS=$(wildcard **/*_gomock.go)
GOFLAGS=
GOTESTFLAGS=-timeout 20s

.PHONY: clean test coverage generate release debug

default: $(EXEC)

debug: GOFLAGS=-race
debug: $(EXEC)

test:
	go test ./.../... -race $(GOTESTFLAGS)

coverage: $(COVERPROF)
	go test ./.../... -coverprofile=$(COVERPROF)
	go tool cover -func=$(COVERPROF)

coverage-html: $(COVERPROF)
	go test ./.../... -coverprofile=$(COVERPROF)
	go tool cover -html=$(COVERPROF)

format:
	@ go fmt ./.../...

# TODO migrate protoc calls to go geneate statements
generate:
	@ rm -rf $(PROTO) $(GOMOCKS)
	@ protoc $(LIBRPC) --go_out=plugins=grpc:.
	@ go generate ./.../...

install:
	@ go install ./...

clean:
	@ go clean ./.../...
	@rm -rf $(BIN) $(TARGET)

$(BIN):
	@mkdir $(BIN)

$(BIN)/%: $(EXECSRC) $(LIBRPC) $(LIBSRC) $(BIN)
	@cd $(patsubst bin/%,cmd/%,$@) && go build $(GOFLAGS) -o ../../$@

make_release:
	@ mkdir -p $(TARGET)/$(TARGET_OS)_$(TARGET_ARCH)
	@ GOARCH=$(TARGET_ARCH) $(TARGET_ARCH_FLAGS) GOOS=$(TARGET_OS) go build -o `pwd`/$(TARGET)/$(TARGET_OS)_$(TARGET_ARCH) ./... 
	@ cp -R deploy $(TARGET)/$(TARGET_OS)_$(TARGET_ARCH)
	@ cp deploy/Makefile $(TARGET)/$(TARGET_OS)_$(TARGET_ARCH)
	@ rm $(TARGET)/$(TARGET_OS)_$(TARGET_ARCH)/deploy/Makefile
	@ rm -rf $(TARGET)/$(TARGET_OS)_$(TARGET_ARCH)/deploy/logd/deps

ARM=arm
AMD=amd64
GOOS=linux
release: default
	@ rm -rf $(TARGET)
	@ TARGET_OS=linux TARGET_ARCH=arm TARGET_ARCH_FLAGS=GOARM=7 $(MAKE) make_release
	@ TARGET_OS=linux TARGET_ARCH=amd64 $(MAKE) make_release
	@ TARGET_OS=darwin TARGET_ARCH=amd64 $(MAKE) make_release
	@ cd $(TARGET) && tar -czvf blue-release-`git describe --tags --dirty`.tar.gz *

dist: release
	@ ./dist.sh
