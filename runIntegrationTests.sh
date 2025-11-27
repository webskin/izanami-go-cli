#!/bin/bash

# Integration tests for iz CLI against a local Izanami server
# Alias: izit
#
# Usage:
#   izit              - Run all integration tests
#   izit Tenants      - Run only TestIntegration_Tenants* tests
#   izit Projects     - Run only TestIntegration_Projects* tests

FILTER="${1:-}"
PATTERN="TestIntegration${FILTER:+_$FILTER}"

IZ_TEST_BASE_URL=http://localhost:9000 \
IZ_TEST_USERNAME=RESERVED_ADMIN_USER \
IZ_TEST_PASSWORD=password \
go test ./internal/cmd/... -run "$PATTERN" -v
