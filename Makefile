VERSION = 0.1.0
TAG = latest
PREFIX ?= quay.io/samsung_cnct/lbex
SRC = services.go endpoints.go task_queue.go controller.go main.go

all: push osx-lbex

lbex: $(SRC)
	CGO_ENABLED=0 GOOS=linux godep go build -a -ldflags '-w' -o $@ $(SRC)

osx-lbex: $(SRC)
	CGO_ENABLED=0 GOOS=darwin go build -a -ldflags "-w" -o $@ $(SRC)

container: lbex
	docker build --rm --pull --tag $(PREFIX):$(TAG) .

push: container
	docker push $(PREFIX):$(TAG)

clean:
	docker rmi -f $(PREFIX):$(TAG) || true
	rm -f lbex
	rm -f osx-lbex

.PHONY: all container push clean
