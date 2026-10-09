SFOS_VERSION := 5.2.0.15
SFOS_ARCH ?= aarch64
GOOS := linux
GOARCH := arm64
SDK_IMAGE := coderus/sailfishos-platform-sdk:$(SFOS_VERSION)
SDK_TARGET := SailfishOS-$(SFOS_VERSION)-$(SFOS_ARCH)
RPM_OUT := rpm-out
BINARY := inari-blue-notifier-daemon

.PHONY: build rpm pull

build:
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build -buildvcs=false -mod=readonly -trimpath -o $(BINARY) .

pull:
	docker pull $(SDK_IMAGE)

rpm: build pull
	mkdir -p $(RPM_OUT)
	chmod 0777 $(RPM_OUT)
	docker run --rm \
		--volume "$(CURDIR):/share" \
		$(SDK_IMAGE) /bin/bash -ec '\
			mkdir build; \
			cd build; \
			cp -r /share/* .; \
			mb2 -t "$(SDK_TARGET)" build; \
			cp RPMS/*.rpm /share/$(RPM_OUT)/'
