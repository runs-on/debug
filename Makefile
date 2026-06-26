COMMAND := "."
UPX_BIN := $(shell command -v upx 2> /dev/null)

.PHONY: help
help:
	@echo 'Usage:'
	@echo '  make test'
	@echo '  make build'
	@echo '  make js'

.PHONY: test
test:
	go test ./...

.PHONY: js
js:
	rm -f index.js
	cp index.template.js index.js

.PHONY: runs-on-debug-linux-amd64
runs-on-debug-linux-amd64: _require-upx
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -installsuffix static -o "runs-on-debug-linux-amd64" $(COMMAND)
	upx -q -9 "runs-on-debug-linux-amd64"

.PHONY: runs-on-debug-linux-arm64
runs-on-debug-linux-arm64: _require-upx
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -installsuffix static -o "runs-on-debug-linux-arm64" $(COMMAND)
	upx -q -9 "runs-on-debug-linux-arm64"

.PHONY: runs-on-debug-windows-amd64
runs-on-debug-windows-amd64: _require-upx
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -installsuffix static -o "runs-on-debug-windows-amd64.exe" $(COMMAND)
	upx -q -9 "runs-on-debug-windows-amd64.exe"

.PHONY: build
build: runs-on-debug-linux-amd64 runs-on-debug-linux-arm64 runs-on-debug-windows-amd64 js

.PHONY: _require-upx
_require-upx:
ifndef UPX_BIN
	$(error 'upx is not installed')
endif
