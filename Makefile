bin: bin/preflight-dns_darwin_amd64 bin/preflight-dns_linux_amd64 bin/preflight-dns_windows_amd64.exe
bin: bin/preflight-dns_darwin_arm64 bin/preflight-dns_linux_arm64 bin/preflight-dns_windows_arm64.exe

bin/preflight-dns_darwin_amd64:
	@mkdir -p bin
	@echo "Compiling preflight-dns..."
	GOOS=darwin GOARCH=amd64 go build -o $@ cmd/preflight-dns/*.go

bin/preflight-dns_darwin_arm64:
	@mkdir -p bin
	@echo "Compiling preflight-dns..."
	GOOS=darwin GOARCH=arm64 go build -o $@ cmd/preflight-dns/*.go

bin/preflight-dns_linux_amd64:
	@mkdir -p bin
	@echo "Compiling preflight-dns..."
	GOOS=linux GOARCH=amd64 go build -o $@ cmd/preflight-dns/*.go

bin/preflight-dns_linux_arm64:
	@mkdir -p bin
	@echo "Compiling preflight-dns..."
	GOOS=linux GOARCH=arm64 go build -o $@ cmd/preflight-dns/*.go

bin/preflight-dns_windows_amd64.exe:
	@mkdir -p bin
	@echo "Compiling preflight-dns..."
	GOOS=windows GOARCH=amd64 go build -o $@ cmd/preflight-dns/*.go

bin/preflight-dns_windows_arm64.exe:
	@mkdir -p bin
	@echo "Compiling preflight-dns..."
	GOOS=windows GOARCH=arm64 go build -o $@ cmd/preflight-dns/*.go

.PHONY: install
install: bin
	@echo "Installing preflight-dns..."
	@scp bin/preflight-dns_$$(go env GOOS)_$$(go env GOARCH) /usr/local/bin/preflight-dns