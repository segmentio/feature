branch ?= $(shell git rev-parse --abbrev-ref HEAD)
commit ?= $(shell git rev-parse --short=7 HEAD)
version ?= $(subst /,-,$(branch))-$(commit)
image ?= 528451384384.dkr.ecr.us-west-2.amazonaws.com/feature:$(version)

feature: vendor $(wildcard *.go) $(wildcard ./cmd/feature/*.go)
	CGO_ENABELD=0 go build -mod=vendor ./cmd/feature

docker: vendor
	docker build -t feature .

publish: docker
	docker tag feature $(image)
	docker push $(image)

vendor: ./vendor/modules.txt

./vendor/modules.txt: go.mod go.sum
	go mod vendor

.PHONY: docker publish vendor
