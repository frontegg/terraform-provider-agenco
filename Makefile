default: build

build:
	go build -o terraform-provider-agenco

install: build
	mkdir -p ~/.terraform.d/plugins/registry.terraform.io/frontegg/agenco/0.0.1/$$(go env GOOS)_$$(go env GOARCH)
	cp terraform-provider-agenco ~/.terraform.d/plugins/registry.terraform.io/frontegg/agenco/0.0.1/$$(go env GOOS)_$$(go env GOARCH)/

test:
	go test -v ./...

testacc:
	TF_ACC=1 go test -v ./... -timeout 120m

fmt:
	go fmt ./...

vet:
	go vet ./...

tidy:
	go mod tidy

# The registry Overview comes from docs/index.md, which tfplugindocs builds from
# templates/index.md.tmpl — not from README.md. Derive that template from the README first so the
# two cannot drift.
docs:
	python3 scripts/gen-index-template.py
	tfplugindocs generate --provider-name agenco

# Writes a Terraform CLI config pointing frontegg/agenco at this working copy, so plan and
# apply use the locally built binary instead of the registry. No terraform init required.
dev.tfrc:
	@printf 'provider_installation {\n  dev_overrides {\n    "frontegg/agenco" = "%s"\n  }\n  direct {}\n}\n' "$(CURDIR)" > dev.tfrc
	@echo "wrote dev.tfrc -> $(CURDIR)"

dev-override: dev.tfrc

smoke-plan: build dev.tfrc
	cd local-test && TF_CLI_CONFIG_FILE=../dev.tfrc terraform plan

smoke-apply: build dev.tfrc
	cd local-test && TF_CLI_CONFIG_FILE=../dev.tfrc terraform apply

smoke-destroy: build dev.tfrc
	cd local-test && TF_CLI_CONFIG_FILE=../dev.tfrc terraform destroy

# Migrates a fixture from frontegg/agentlink 0.4.7 into this working copy and asserts that nothing
# is recreated. migration-validate needs no credentials; migration-test writes to a live vendor.
migration-validate:
	./test/migration/run.sh validate

migration-test:
	./test/migration/run.sh

.PHONY: build install test testacc fmt vet tidy docs dev-override smoke-plan smoke-apply \
	smoke-destroy migration-validate migration-test
