.PHONY: run check

run:
	@if [ -z "$$GEMINI_API_KEY" ]; then \
		echo "Error: GEMINI_API_KEY is not set." >&2; \
		exit 1; \
	fi
	go run .

check:
	go test ./...
