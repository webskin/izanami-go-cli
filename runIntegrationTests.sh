#!/bin/bash

# Integration tests for iz CLI against a local Izanami server
# Alias: izit

IZ_TEST_BASE_URL=http://localhost:9000 \
IZ_TEST_USERNAME=RESERVED_ADMIN_USER \
IZ_TEST_PASSWORD=password \
go test ./internal/cmd/... -run TestIntegration -v
