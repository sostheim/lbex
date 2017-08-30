NAME      := lbex
VERSION   := 0.1.9
TYPE      := alpha
COMMIT    := $(shell git rev-parse HEAD)
IMAGE     := quay.io/samsung_cnct/lbex
TAG       ?= latest
godep=GOPATH=$(shell godep path):${GOPATH}

build:
	@go build -ldflags "-X main.LbexMajorMinorPatch=$(VERSION) \
		-X main.LbexType=$(TYPE) \
		-X main.LbexGitCommit=$(COMMIT)"

compile: deps
	@rm -rf build/
	@$(GODEP) gox -ldflags "-X main.LbexMajorMinorPatch=$(VERSION) \
									-X main.LbexType=$(TYPE) \
									-X main.LbexGitCommit=$(COMMIT) -w" \
	-osarch="linux/386" \
	-osarch="linux/amd64" \
	-osarch="darwin/amd64" \
	-output "build/{{.OS}}_{{.Arch}}/$(NAME)" \
	./...

install:
	@godep go install -ldflags "-X main.LbexMajorMinorPatch=$(VERSION) \
									-X main.LbexType=$(TYPE) \
									-X main.LbexGitCommit=$(COMMIT) -w"

deps:
	go get github.com/mitchellh/gox
	go get github.com/tools/godep

dist: compile
	$(eval FILES := $(shell ls build))
	@rm -rf dist && mkdir dist
	@for f in $(FILES); do \
		(cp $(shell pwd)/nginx/*.tmpl $(shell pwd)/build/$$f); \
		(cd $(shell pwd)/build/$$f && tar -cvzf ../../dist/$$f.tar.gz *); \
		(cd $(shell pwd)/dist && shasum -a 512 $$f.tar.gz > $$f.sha512); \
		echo $$f; \
	done

container:
	@$(GODEP) gox -ldflags "-X main.LbexMajorMinorPatch=$(VERSION) \
									-X main.LbexType=$(TYPE) \
									-X main.LbexGitCommit=$(COMMIT) -w" \
	-osarch="linux/amd64" \
	-output "build/{{.OS}}_{{.Arch}}/$(NAME)" \
	./...
	docker build --rm --pull --tag $(IMAGE):$(TAG) .

tag: container
	docker tag $(IMAGE):$(TAG) $(IMAGE):$(COMMIT)

push: tag
	docker push $(IMAGE):$(COMMIT)
	docker push $(IMAGE):$(TAG)

release: dist push
	@latest_tag=$$(git describe --tags `git rev-list --tags --max-count=1`); \
	comparison="$$latest_tag..HEAD"; \
	if [ -z "$$latest_tag" ]; then comparison=""; fi; \
	changelog=$$(git log $$comparison --oneline --no-merges --reverse); \
	github-release samsung-cnct/$(NAME) $(VERSION) "$$(git rev-parse --abbrev-ref HEAD)" "**Changelog**<br/>$$changelog" 'dist/*'; \
	git pull

.PHONY: build compile install deps dist release push tag container