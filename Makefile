SHELL   := bash
NAME    := navikt/named
LATEST  := ${NAME}:latest
GLIDE   := docker run --rm -v ${PWD}:/go/src/named -w /go/src/named navikt/glide glide
GO_IMG  := golang:1.8
GO      := docker run --rm -v ${PWD}:/go/src/named -w /go/src/named ${GO_IMG} go
LDFLAGS := -X name/api/version.Version=$(shell /bin/cat ./version)

.PHONY: dockerhub-release install test linux bump tag cli cli-dist build docker-build push-dockerhub docker-minikube-build helm-upgrade

dockerhub-release: install test linux bump tag docker-build push-dockerhub
minikube: linux docker-minikube-build helm-upgrade

bump:
	/bin/bash bump.sh

tag:
	git tag -a $(shell /bin/cat ./version) -m "auto-tag from Makefile [skip ci]" && git push --tags

install:
	${GLIDE} install --strip-vendor

test:
	${GO} test ./api/

cli:
	${GO} build -ldflags='$(LDFLAGS)' -o nais ./cli


build:
	${GO} build -o named

linux:
	docker run --rm \
		-e GOOS=linux \
		-e CGO_ENABLED=0 \
		-v ${PWD}:/go/src/named \
		-w /go/src/named ${GO_IMG} \
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
