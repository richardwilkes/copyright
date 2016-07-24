QUIET = @
GIT_VERSION := $(shell if which git 2>&1 > /dev/null; then echo $$(git rev-parse HEAD)-$$(if [ -z "$$(git status --porcelain)" ]; then echo clean; else echo dirty; fi); else echo Unknown; fi)

help: FORCE
	@echo "Available targets:"
	@echo "  aligncheck  Run aligncheck across the source base."
	@echo "  build       Generate necessary source files and build. (default)"
	@echo "  copyright   Build and install copyright."
	@echo "  install     Build and install the library."
	@echo "  lint        Run golint across the source base."

aligncheck: FORCE
	$(QUIET)find . -type d ! -ipath "./.git*" -exec aligncheck \{\} \;

build: FORCE
	$(QUIET)go generate; go build -v

copyright: FORCE
	$(eval VERSION=$(shell genversion --major 1))
	$(QUIET)touch copyright.go; go install -v -ldflags "-X main.version=$(VERSION) -X github.com/richardwilkes/cmdline.GitVersion=$(GIT_VERSION)"

install: FORCE
	$(QUIET)go generate; go install -v

lint: FORCE
	$(QUIET)find . -type d ! -ipath "./.git*" -exec golint \{\} \;

FORCE:
