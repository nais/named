SHELL   := bash
NAME    := navikt/named
LATEST  := ${NAME}:latest
DEP   := docker run --rm -v ${PWD}:/go/src/github.com/nais/named -w /go/src/github.com/nais/named navikt/dep ensure
GO_IMG  := golang:1.9
GO      := docker run --rm -v ${PWD}:/go/src/github.com/nais/named -w /go/src/github.com/nais/named ${GO_IMG} go
LDFLAGS := -X github.com/nais/named/api/version.Revision=$(shell git rev-parse --short HEAD) -X github.com/nais/named/api/version.Version=$(shell /bin/cat ./version)

.PHONY: dockerhub-release install test linux bump tag cli cli-dist build docker-build push-dockerhub docker-minikube-build helm-upgrade

dockerhub-release: install test bump linux tag docker-build push-dockerhub
minikube: linux docker-minikube-build helm-upgrade

bump:
	/bin/bash bump.sh

tag:
	git tag -a $(shell /bin/cat ./version) -m "auto-tag from Makefile [skip ci]" && git push --tags

install:
	${DEP}

test:
	${GO} test ./api/ ./cli/

cli:
	${GO} build -ldflags='$(LDFLAGS)' -o name ./cli

cli-dist:
	docker run --rm -v \
		${PWD}\:/go/src/github.com/nais/named \
		-w /go/src/github.com/nais/named \
		-e GOOS=linux \
		-e GOARCH=amd64 \
		${GO_IMG} go build -o name-linux-amd64 -ldflags="-s -w $(LDFLAGS)" ./cli/name.go
	sudo xz name-linux-amd64

	docker run --rm -v \
		${PWD}\:/go/src/github.com/nais/named \
		-w /go/src/github.com/nais/named \
		-e GOOS=darwin \
		-e GOARCH=amd64 \
		${GO_IMG} go build -o name-darwin-amd64 -ldflags="-s -w $(LDFLAGS)" ./cli/name.go
	sudo xz name-darwin-amd64

	docker run --rm -v \
		${PWD}\:/go/src/github.com/nais/named \
		-w /go/src/github.com/nais/named \
		-e GOOS=windows \
		-e GOARCH=amd64 \
		${GO_IMG} go build -o name-windows-amd64 -ldflags="-s -w $(LDFLAGS)" ./cli/name.go
	zip -r name-windows-amd64.zip name-windows-amd64
	sudo rm name-windows-amd64

build:
	${GO} build -o named

linux:
	docker run --rm \
		-e GOOS=linux \
		-e CGO_ENABLED=0 \
		-v ${PWD}:/go/src/github.com/nais/named \
		-w /go/src/github.com/nais/named ${GO_IMG} \
		go build -a -installsuffix cgo -ldflags '-s $(LDFLAGS)' -o named

docker-minikube-build:
	@eval $$(minikube docker-env) ;\
	docker image build -t ${NAME}:$(shell /bin/cat ./version) -t ${NAME} -t ${LATEST} -f Dockerfile --no-cache .

docker-build:
	docker image build -t ${NAME}:$(shell /bin/cat ./version) -t named -t ${NAME} -t ${LATEST} -f Dockerfile .

push-dockerhub:
	docker image push ${NAME}:$(shell /bin/cat ./version)

helm-upgrade:
	helm delete named; helm upgrade -i named helm/named --set image.version=$(shell /bin/cat ./version)
