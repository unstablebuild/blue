TARGET=bin
LIBSRC=$(wildcard **/*.go) $(wildcard **/**/*.go) $(wildcard **/**/**/*.go)
LIBRPC=$(wildcard rpc/*.proto)
EXECSRC=$(wildcard cmd/**/*.go)
EXECDIRS=$(sort $(dir $(EXECSRC)))
EXEC=$(patsubst cmd/%/,$(TARGET)/%,$(EXECDIRS))
COVERPROF=test.coverprofile
CODEGEN=rpc/*.pb.go

.PHONY: clean test coverage rpc

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
	@rm -rf $(TARGET)

$(TARGET):
	@mkdir $(TARGET)

$(TARGET)/%: $(EXECSRC) $(LIBRPC) $(LIBSRC) $(TARGET)
	@cd $(patsubst bin/%,cmd/%,$@) && go build $(CFLAGS) -o ../../$@
