IMAGE ?= starkandwayne/gluon-apparatus
TAG   ?= latest

docker-build:
	docker build -t $(IMAGE):$(TAG) .
docker-push:
	docker push $(IMAGE):$(TAG)

release: docker-build docker-push
