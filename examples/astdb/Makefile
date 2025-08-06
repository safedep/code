BIN_DIR := bin
BIN := $(BIN_DIR)/astdb

all: create_bin astdb

.PHONY: create_bin
create_bin:
	mkdir -p $(BIN_DIR)

.PHONY: astdb
astdb:
	go build -o $(BIN) main.go

.PHONY: clean
clean:
	rm -rf $(BIN_DIR)

