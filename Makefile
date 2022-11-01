BIN=bin
TARGET=target
LIBSRC=$(wildcard **/*.go) $(wildcard **/**/*.go) $(wildcard **/**/**/*.go)
EXECSRC=$(wildcard cmd/**/*.go)
EXECDIRS=$(sort $(dir $(EXECSRC)))
EXEC=$(patsubst cmd/%/,$(BIN)/%,$(EXECDIRS))
COVERPROF=test.coverprofile
LIBRPC=$(wildcard **/**/*.proto)
GOFLAGS="-ldflags=-X main.Tag=$$(git describe --tags) -X main.Commit=$$(git rev-parse --short HEAD)"
GOTESTFLAGS=-timeout 120s

.PHONY: clean test coverage generate release debug

default: CGO_ENABLED=CGO_ENABLED=0
default: $(EXEC)

debug: GOFLAGS=-race
debug: CGO_ENABLED=CGO_ENABLED=1
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

generate:
	@ go generate ./.../...

install:
	@ go install ./...

clean:
	@ go clean ./.../...
	@rm -rf $(BIN) $(TARGET)

$(BIN):
	@mkdir $(BIN)

$(BIN)/%: $(EXECSRC) $(LIBRPC) $(LIBSRC) $(BIN)
	@cd $(patsubst bin/%,cmd/%,$@) && $(CGO_ENABLED) go build $(GOFLAGS) -o ../../$@

make_release:
	@ mkdir -p $(TARGET)/$(TARGET_OS)_$(TARGET_ARCH)
	@ CGO_ENABLED=0 GOARCH=$(TARGET_ARCH) $(TARGET_ARCH_FLAGS) GOOS=$(TARGET_OS) go build $(GOFLAGS) -o `pwd`/$(TARGET)/$(TARGET_OS)_$(TARGET_ARCH) ./... 
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
