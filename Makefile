NAME      := lbex
VERSION   := 0.1.1
TYPE      := alpha
COMMIT    := $(shell git rev-parse HEAD)
IMAGE     := quay.io/samsung_cnct/lbex
godep=GOPATH=$(shell godep path):${GOPATH}

build: deps
  @godep go build -ldflags "-X github.com/samsung-cnct/lbex/main.LbexMajorMinorPatch=$(VERSION) \
    -X github.com/samsung-cnct/lbex/main.LbexType=$(TYPE) \
    -X github.com/samsung-cnct/lbex/main.LbexGitCommit=$(COMMIT)"

compile:
  @rm -rf build/
  @$(GODEP) gox -ldflags "-X github.com/samsung-cnct/lbex/main.LbexMajorMinorPatch=$(VERSION) \
                  -X github.com/samsung-cnct/lbex/main.LbexType=$(TYPE) \
                  -X github.com/samsung-cnct/lbex/main.LbexGitCommit=$(COMMIT) -w" \
  -osarch="linux/386" \
  -osarch="linux/amd64" \
  -osarch="darwin/amd64" \
  -output "build/{{.OS}}_{{.Arch}}/$(NAME)" \
  ./...

install:
  @godep go install -ldflags "-X github.com/samsung-cnct/lbex/main.LbexMajorMinorPatch=$(VERSION) \
                  -X github.com/samsung-cnct/lbex/main.LbexType=$(TYPE) \
                  -X github.com/samsung-cnct/lbex/main.LbexGitCommit=$(COMMIT) -w"

deps:
  go get github.com/mitchellh/gox
  go get github.com/tools/godep

dist: compile
  $(eval FILES := $(shell ls build))
  @rm -rf dist && mkdir dist
  @for f in $(FILES); do \
    (cd $(shell pwd)/build/$$f && tar -cvzf ../../dist/$$f.tar.gz *); \
    (cd $(shell pwd)/dist && shasum -a 512 $$f.tar.gz > $$f.sha512); \
    echo $$f; \
  done

container: compile
  docker build --rm --pull --tag $(PREFIX):latest .

tag: container
  docker tag $(IMAGE):latest $(IMAGE):$(COMMIT)

push: tag
  docker push $(IMAGE):$(COMMIT)
  docker push $(IMAGE):latest

release: dist push
  @latest_tag=$$(git describe --tags `git rev-list --tags --max-count=1`); \
  comparison="$$latest_tag..HEAD"; \
  if [ -z "$$latest_tag" ]; then comparison=""; fi; \
  changelog=$$(git log $$comparison --oneline --no-merges --reverse); \
  github-release samsung-cnct/$(NAME) $(VERSION) "$$(git rev-parse --abbrev-ref HEAD)" "**Changelog**<br/>$$changelog" 'dist/*'; \
  git pull

.PHONY: build compile install deps dist release push tag container