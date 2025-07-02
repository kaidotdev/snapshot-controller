.DEFAULT_GOAL := all

.PHONY: all
all:
	@go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.14.0
	@$(shell go env GOPATH)/bin/controller-gen paths="./..." object crd:crdVersions=v1,allowDangerousTypes=true output:crd:artifacts:config=manifests/crd

.PHONY: dev
dev:
	@kind create cluster --name snapshot-controller --config kind.yaml
	@trap 'kind delete cluster --name snapshot-controller' EXIT ERR INT; skaffold dev --port-forward
