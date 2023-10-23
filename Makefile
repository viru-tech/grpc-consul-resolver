GOLANG_IMAGE = golang:1.21
GOLANG_CI_IMAGE = golangci/golangci-lint:v1.55

.PHONY: lint
lint:
	$(V)golangci-lint run

.PHONY: docker-lint
docker-lint: DOCKER_IMAGE = $(GOLANG_CI_IMAGE)
docker-lint:
	$(call run_in_docker,make lint)

.PHONY: test
test: GO_TEST_FLAGS += -race
test:
	$(V)go test $(GO_TEST_FLAGS) --tags=$(GO_TEST_TAGS) ./...

.PHONY: docker-test
docker-test: DOCKER_IMAGE = $(GOLANG_IMAGE)
docker-test:
	$(call run_in_docker, make test)

CURR_REPO := /$(notdir $(PWD))
define run_in_docker
	$(V)docker run --rm \
		-v $(PWD):$(CURR_REPO) \
		-w $(CURR_REPO) \
		$(DOCKER_FLAGS) \
		$(DOCKER_IMAGE) $1
endef
