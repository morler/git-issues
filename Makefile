build:
	go build -o issues ./cmd/issues

install: build
	cp issues /usr/local/bin/issues

test:
	go test ./...

fmt-check:
	@list() { \
		git ls-files --cached --others --exclude-standard -z '*.go' 2>/dev/null || \
		find . -name '*.go' -print0; \
	}; \
	if [ "$$(list | wc -c)" -eq 0 ]; then \
		echo "No Go files to check."; \
		exit 0; \
	fi; \
	unformatted=$$(list | xargs -0 gofmt -l) || { echo "gofmt failed"; exit 1; }; \
	if [ -n "$$unformatted" ]; then \
		echo "Unformatted files:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi

lint: fmt-check
	go vet ./...
	golangci-lint run ./...

clean:
	rm -f issues

setup-agent:
	@echo "Installing git-issues binary..."
	go install github.com/steviee/git-issues/cmd/issues@latest
	@echo "Installing Claude Code skill..."
	@mkdir -p ~/.claude/skills/git-issues
	@cp skills/git-issues/SKILL.md ~/.claude/skills/git-issues/SKILL.md
	@echo "Done. Skill available as /git-issues in Claude Code."

.PHONY: build install test fmt-check lint clean setup-agent
