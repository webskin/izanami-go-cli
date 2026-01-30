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

# PostgreSQL DSN (Data Source Name) for connection cleanup after tenant deletion
# Override with: IZ_TEST_DB_DSN=postgres://user:pass@host:port/dbname?sslmode=disable

IZ_TEST_BASE_URL=http://localhost:9000 \
IZ_TEST_USERNAME=RESERVED_ADMIN_USER \
IZ_TEST_PASSWORD=password \
IZ_TEST_DB_DSN=${IZ_TEST_DB_DSN:-postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable} \
go test -tags=integration -count=1 ./internal/cmd/... -run "$PATTERN" -v
