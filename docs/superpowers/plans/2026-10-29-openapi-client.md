# OpenAPI client migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** Replace handwritten API operations/models with generated Go code without changing the Terraform interface.

**Architecture:** A reviewed provider-specific OpenAPI document generates operations and allowlisted models. The existing client package adapts those operations while retaining authentication, bounded decoding and safe errors.

**Tech Stack:** Go 1.27.3, OpenAPI 3.2.1, oapi-codegen v2.8.1, Terraform Plugin Framework.

**Spec:** `docs/superpowers/specs/2026-10-29-openapi-client-design.md`

## Global Constraints

- qBittorrent 5.3.0 / Web API 2.16.2 only; upstream commit `9213075dd5f7b352208361c6a84c1c115efc898e`.
- Preserve Terraform attributes/state, optional false/zero semantics, auth methods, proxy prefixes and safe retries.
- Never expose unknown preference fields, credentials or response bodies in diagnostics.
- Generated source is checked in; CI rejects generation drift. No live application mutations.

## Review Focus

- Special characters in form values must round-trip without splitting parameters.
- Empty category paths and explicit false/zero preferences must be sent, not dropped.
- Non-JSON content types must not change the existing bounded decoding contract.
- Oversized or malformed replies must fail without exposing server data.
- Session expiry must not replay mutations or retry reads indefinitely.

### Task 1: Specification and generated operations

**Files:** `api/qbittorrent.yaml`, `api/oapi-codegen.yaml`, `internal/client/openapi/generate.go`, `internal/client/openapi/client.gen.go`, `GNUmakefile`, `.github/workflows/test.yml`, `go.mod`, `go.sum`.

**Interfaces:** Generates `openapi.Preferences`, `openapi.Category` and typed operations for the nine existing endpoints. Existing client models retain Go names through schema extensions.

- [x] Record baseline `go test ./...` result.
- [x] Add a generated-client contract test for form escaping and partial preference JSON; run before generation and observe the missing package failure.
- [x] Derive specification from the exact upstream tag; generate models and client with pinned tooling.
- [x] Run generated contract tests and add `make generate` / `make generate-check` with CI enforcement.

### Task 2: Client adapter migration

**Files:** `internal/client/client.go`, `auth.go`, `preferences.go`, `categories.go`, `client_test.go`.

**Interfaces:** Preserve `New(Config)`, all public client methods, `HTTPError`, `Preferences` and `Category` field names. Generated operations return `*http.Response`; the adapter owns bounded reads and safe diagnostics.

- [x] Add regression cases for special-character forms, empty save paths, explicit false/zero, and oversized replies; run against baseline to establish existing behavior.
- [x] Replace handwritten request construction and form maps with generated operations and generated model aliases.
- [x] Run `go test ./...` and `go test -race ./...`; preserve existing session/timeout/redirect/malformed-response tests.

### Task 3: Verification and developer workflow

**Files:** `README.md`, `docs/development.md`, `docs/verification.md`, plan status.

**Interfaces:** `make generate-check` validates the committed artifact; existing docs and disposable acceptance commands remain valid.

- [x] Document schema provenance, subset scope, regeneration and retained adapter responsibilities.
- [x] Verify generation drift detection by introducing a temporary generated-file change and restoring it.
- [x] Run vet/build, docs checks and disposable acceptance for Terraform/OpenTofu where available.
- [x] Review the full diff and record verification results and limitations; commit the completed migration without publishing a release.
