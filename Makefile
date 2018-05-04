.PHONY: build release
.DEFAULT_GOAL := build

GITHUB_ORG=mailgun
GITHUB_REPO=k8-entrypoint

UPSTREAM=$(GITHUB_ORG)/$(GITHUB_REPO)

image:
	docker build --no-cache -t k8-entrypoint:latest .

build:
	GOOS=linux GOARCH=amd64 go build -o k8-entrypoint ./cmd/k8-entrypoint

release: build
	@if [ "$(VERSION)" = "" ]; then \
		echo " # Makefile currently has version: VERSION=$(VERSION)"; \
		echo " # 'VERSION' variable not set! To preform a release do the following"; \
		echo "  git tag v1.0.0"; \
		echo "  git push --tags"; \
		echo "  make release VERSION=v1.0.0"; \
		echo ""; \
		exit 1; \
	fi
	@if ! which github-release 2>&1 >> /dev/null; then \
		echo " # github-release not found in path; install and create a github token with 'repo' access"; \
		echo " # See (https://help.github.com/articles/creating-an-access-token-for-command-line-use)"; \
		echo " go get github.com/aktau/github-release"; \
		echo " export GITHUB_TOKEN=<your-token>";\
		echo ""; \
		exit 1; \
	fi
	@github-release release \
		--user $(GITHUB_ORG) \
		--repo $(GITHUB_REPO) \
		--tag $(VERSION)
	@github-release upload \
		--user $(GITHUB_ORG) \
		--repo $(GITHUB_REPO) \
		--tag $(VERSION) \
		--name "k8-entrypoint" \
		--file k8-entrypoint
