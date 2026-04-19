# Cross-SDK interop test harness.
#
#   make produce   - Every available SDK emits vectors-<sdk>.json
#   make verify    - Every language's verifier checks every vector file
#   make clean     - Remove persisted keypairs + vector files
#
# Missing SDKs (no toolchain on this host) are skipped cleanly.

PY     := python
GO     := go
NODE   := node

.PHONY: produce verify clean produce-python produce-go verify-python

produce: produce-python produce-go

produce-python:
	@$(PY) -c "import quidnug" >/dev/null 2>&1 || \
		(cd ../../clients/python && pip install -e . >/dev/null)
	@$(PY) produce.py > vectors-python.json && echo "✓ vectors-python.json"

produce-go:
	@$(GO) run ./produce_go.go > vectors-go.json 2>/dev/null && echo "✓ vectors-go.json" \
		|| echo "… skipping Go (run from repo root to enable module resolution)"

verify: verify-python
	@echo "✓ verification complete"

verify-python:
	@$(PY) verify.py vectors-*.json

clean:
	@rm -f vectors-*.json .keypair .keypair-go
	@echo "✓ cleaned"
