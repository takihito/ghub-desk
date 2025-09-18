package mcp

import (
	"encoding/json"
	"fmt"
)

// Standard JSON-RPC/MCP error codes used in this project.
const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602

	CodeServerError      = -32000
	CodeDBNotInitialized = -32001
	CodeNotImplemented   = -32002
	CodeConfigRequired   = -32003
	CodeClientInitFailed = -32004

	CodeForbidden = 403
)

func ok(id json.RawMessage, result any) *Response {
	return &Response{JSONRPC: "2.0", ID: id, Result: result}
}

func rpcErr(id json.RawMessage, code int, msg string, data any) *Response {
	return &Response{JSONRPC: "2.0", ID: id, Error: &RPCError{Code: code, Message: msg, Data: data}}
}

func errInvalidParams(id json.RawMessage, data any) *Response {
	return rpcErr(id, CodeInvalidParams, "invalid params", data)
}

func errMethodNotFound(id json.RawMessage, method string) *Response {
	return rpcErr(id, CodeMethodNotFound, fmt.Sprintf("method %s not found", method), nil)
}

func errDBNotInitialized(id json.RawMessage) *Response {
	return rpcErr(id, CodeDBNotInitialized, "database not initialized", nil)
}

func errForbidden(id json.RawMessage, msg string) *Response {
	return rpcErr(id, CodeForbidden, msg, nil)
}

func errConfigRequired(id json.RawMessage, op string) *Response {
	return rpcErr(id, CodeConfigRequired, fmt.Sprintf("configuration required for %s", op), nil)
}

func errClientInitFailed(id json.RawMessage, err error) *Response {
	return rpcErr(id, CodeClientInitFailed, "failed to init GitHub client", err.Error())
}

func errServer(id json.RawMessage, msg string, err error) *Response {
	var data any
	if err != nil {
		data = err.Error()
	}
	return rpcErr(id, CodeServerError, msg, data)
}
