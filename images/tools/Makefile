REGISTRY?=icr.io/ai-services-private
IMAGE=tools
TAG?=latest
SR_VERSION=2.2.5-2

CONTAINER_BUILDER?=podman
build:
	$(CONTAINER_BUILDER) build --build-arg SR_VERSION=$(SR_VERSION) -t $(REGISTRY)/$(IMAGE):$(TAG) .
.PHONY: build

push:
	$(CONTAINER_BUILDER) push $(REGISTRY)/$(IMAGE):$(TAG)
.PHONY: push
