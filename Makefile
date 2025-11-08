IMAGE_NAME := kube-janitor
IMAGE_TAG := latest
DEPLOYMENT := kube-janitor
NIX_RESULT := result

.PHONY: all build import deploy restart logs dev clean

all: dev

build:
	nix build

import: build
	sudo k3s ctr images import $(shell readlink -f $(NIX_RESULT))

deploy:
	kubectl apply -k k8s

restart:
	kubectl rollout restart deploy/$(DEPLOYMENT)

logs:
	kubectl logs -f deploy/$(DEPLOYMENT)

dev: build import deploy restart logs

clean:
	@echo "Deleting deployment $(DEPLOYMENT) from Kubernetes..."
	kubectl delete deployments $(DEPLOYMENT) --ignore-not-found
	@echo "Removing Nix build artifacts..."
	rm -rf $(NIX_RESULT)
