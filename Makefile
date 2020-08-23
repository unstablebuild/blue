BIN=bin
TARGET=target
LIBSRC=$(wildcard **/*.go) $(wildcard **/**/*.go) $(wildcard **/**/**/*.go)
LIBRPC=$(wildcard rpc/*.proto)
EXECSRC=$(wildcard cmd/**/*.go)
EXECDIRS=$(sort $(dir $(EXECSRC)))
EXEC=$(patsubst cmd/%/,$(BIN)/%,$(EXECDIRS))
COVERPROF=test.coverprofile
CODEGEN=rpc/*.pb.go

.PHONY: clean test coverage rpc release

default: $(EXEC)

test:
	go test ./.../... -race -coverprofile=$(COVERPROF)

coverage: $(COVERPROF)
	go tool cover -func=$(COVERPROF)

coverage-html: $(COVERPROF)
	go tool cover -html=$(COVERPROF)

format:
	@ go fmt ./.../...

rpc:
	rm -rf $(CODEGEN)
	protoc $(LIBRPC) --go_out=plugins=grpc:.

install:
	@ go install ./...

clean:
	@ go clean ./.../...
	@rm -rf $(BIN) $(TARGET)

$(BIN):
	@mkdir $(BIN)

$(BIN)/%: $(EXECSRC) $(LIBRPC) $(LIBSRC) $(BIN)
	@cd $(patsubst bin/%,cmd/%,$@) && go build $(CFLAGS) -o ../../$@

ARM=arm64
AMD=amd64
GOOS=linux
release: default
	@ rm -rf $(TARGET)
	@ mkdir -p $(TARGET)/$(AMD) $(TARGET)/$(ARM)
	@ GOARCH=$(AMD) GOOS=$(GOOS) go build -o `pwd`/$(TARGET)/$(AMD) ./... 
	@ GOARCH=$(ARM) GOOS=$(GOOS) go build -o `pwd`/$(TARGET)/$(ARM) ./... 
	@ cp -R deploy $(TARGET)/$(ARM)
	@ cp -R deploy $(TARGET)/$(AMD)
	@ cd $(TARGET) && tar -czvf blue-release.tar.gz $(ARM) $(AMD)
