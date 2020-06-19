IMAGE  ?= starkandwayne/gluon:latest
GITHUB := https://github.com/starkandwayne/gluon

.PHONY: default build push
default: build
build:
	docker build \
	  --build-arg BUILD_DATE="$(shell date -u --iso-8601)" \
	  --build-arg VCS_REF="$(shell git rev-parse --short HEAD)" \
	  -t $(IMAGE) .

push: build
	docker push $(IMAGE)
