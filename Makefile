DOCKER_IMG_TAGGED = ellogroup/ello-golang-otel:latest
DOCKER_RUN = docker run --rm --platform linux/amd64 $(DOCKER_IMG_TAGGED)
DOCKER_RUN_SRC = docker run --rm --platform linux/amd64 -v $(CURDIR):/src/app $(DOCKER_IMG_TAGGED)

.PHONY: build-format-test
build-format-test: build format test

.PHONY: build
build: ensure-ai-context
	docker build --platform linux/amd64 -t $(DOCKER_IMG_TAGGED) .
	$(DOCKER_RUN_SRC) go mod tidy

# Initialise the ai-context submodule if it is missing. Skipped in CI —
# pipelines do not need the AI agent context to build or test the app.
.PHONY: ensure-ai-context
ensure-ai-context:
	@if [ ! -f .ai-context/AGENTS.md ] && [ -z "$$CI" ]; then \
		echo "Initialising ai-context submodule..."; \
		git submodule update --init --depth 1 .ai-context || true; \
	fi

# Pull the latest shared AI context. Run this when you want the latest
# standards, conventions, and skills from ellogroup/ai-context. After
# bumping the submodule, sync the Claude Code skill wrappers so any
# newly-added command-bearing skills are exposed. Review the resulting
# changes and commit the new submodule pointer.
.PHONY: sync-ai-context
sync-ai-context:
	git submodule update --remote --merge .ai-context
	$(MAKE) sync-skills
	@echo "ai-context updated. Review .ai-context/ and commit the pointer if appropriate."

# Generate one-line Claude Code skill wrappers under .claude/skills/<name>/
# for every skill in .ai-context/skills/ that declares a `command:` field.
# Idempotent — existing wrappers are skipped, never overwritten or deleted.
.PHONY: sync-skills
sync-skills: ensure-ai-context
	@./scripts/sync-skills.sh

# Seed .agents/memory/ from the latest documentation templates in the
# ai-context submodule. Wraps scripts/init-memory.sh — idempotent,
# existing files are never overwritten. Run once after creating a repo
# from the template.
.PHONY: init-memory
init-memory: ensure-ai-context
	@./scripts/init-memory.sh

.PHONY: format
format:
	$(DOCKER_RUN_SRC) gofmt -w ./
	$(DOCKER_RUN_SRC) go fix ./...
	$(DOCKER_RUN_SRC) goimports -local github.com/ellogroup -w ./

.PHONY: test
test: static-tests unit-tests

.PHONY: static-tests
static-tests:
	$(DOCKER_RUN) golangci-lint config verify
	$(DOCKER_RUN) golangci-lint run -v
	$(DOCKER_RUN) gosec ./...
	$(DOCKER_RUN) govulncheck ./...

.PHONY: unit-tests
unit-tests:
	$(DOCKER_RUN) go test -v -cover ./...
