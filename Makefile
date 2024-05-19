all: local containers

local:
	GOOS="linux" GOARCH="amd64" go build -o bin/magpie-linux-amd64 .
	GOOS="linux" GOARCH="arm64" go build -o bin/magpie-linux-arm64 .
	GOOS="freebsd" GOARCH="amd64" go build -o bin/magpie-freebsd-amd64 .
	GOOS="freebsd" GOARCH="arm64" go build -o bin/magpie-freebsd-arm64 .
containers:
	podman build --jobs=2 --platform=linux/amd64,linux/arm64 --manifest magpie .
containers-publish:
	# you need to `podman login src.tty.cat` first
	podman manifest push localhost/magpie docker://src.tty.cat/home.arpa/magpie:latest
