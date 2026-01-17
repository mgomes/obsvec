.PHONY: build install clean

BINARY_NAME=ofind
BUILD_DIR=./cmd/ofind

# Use Homebrew SQLite to avoid macOS deprecation warnings
SQLITE_PREFIX := $(shell brew --prefix sqlite 2>/dev/null)
ifneq ($(SQLITE_PREFIX),)
    export CGO_CFLAGS := -I$(SQLITE_PREFIX)/include
    export CGO_LDFLAGS := -L$(SQLITE_PREFIX)/lib
endif

build:
	go build -o $(BINARY_NAME) $(BUILD_DIR)

install:
	go install $(BUILD_DIR)

clean:
	rm -f $(BINARY_NAME)
	go clean

deps:
	go mod tidy

run:
	go run $(BUILD_DIR) $(ARGS)
