package ncserver

import (
	"github.com/hilather/go-lab-netconf/internal/domainerr"
	"github.com/hilather/go-lab-netconf/internal/ncrpc"
)

func rpcErr(typ, tag, message string) ncrpc.Error {
	return ncrpc.Error{
		Type:     typ,
		Tag:      tag,
		Severity: "error",
		Message:  message,
	}
}

func okReply(id string) ncrpc.Reply {
	return ncrpc.Reply{MessageID: id, OK: true}
}

func errReply(id string, e ncrpc.Error) ncrpc.Reply {
	if id == "" {
		id = "0"
	}
	return ncrpc.Reply{MessageID: id, Errors: []ncrpc.Error{e}}
}

func mapErr(err error) ncrpc.Error {
	if err == nil {
		return rpcErr("application", "operation-failed", "error")
	}
	de, ok := domainerr.As(err)
	if !ok {
		return rpcErr("application", "operation-failed", err.Error())
	}
	switch de.Code {
	case domainerr.CodeLockDenied:
		return rpcErr("protocol", "lock-denied", de.Message)
	case domainerr.CodeNotWritable:
		return rpcErr("protocol", "operation-not-supported", de.Message)
	case domainerr.CodeNotFound:
		return rpcErr("application", "data-missing", de.Message)
	case domainerr.CodeForbidden, domainerr.CodeUnauthorized:
		return rpcErr("protocol", "access-denied", de.Message)
	case domainerr.CodeValidationFailed:
		tag := "invalid-value"
		for _, v := range de.FieldViolations {
			if v.Code == "unknown-element" {
				tag = "unknown-element"
				break
			}
		}
		return rpcErr("application", tag, de.Message)
	default:
		return rpcErr("application", "operation-failed", de.Message)
	}
}
