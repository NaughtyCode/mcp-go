// Package capi provides a stable, JSON-based wrapper around the public MCP Go
// server API for foreign-function interfaces such as C and C++.
//
// The package intentionally uses opaque handles and JSON payloads instead of
// exposing Go structs across the language boundary. This keeps the ABI small
// while still giving host applications access to MCP server creation,
// registration of tools/resources/prompts, JSON-RPC message handling, and
// built-in transports.
package capi
