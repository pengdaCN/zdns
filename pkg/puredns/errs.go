package puredns

import "errors"

var (
	ErrUnsupportedDnsType = errors.New(`unsupported dns type`)
	ErrUnreachableDns     = errors.New(`unable to find default IP address`)
	ErrInvalidDnsStatus   = errors.New(`invalid dns status`)
	ErrInvalidData        = errors.New(`invalid dns data`)
)
