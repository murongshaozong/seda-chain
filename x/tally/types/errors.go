package types

import "cosmossdk.io/errors"

var (
	// Errors used in filter:
	ErrInvalidFilterType   = errors.Register("tally", 2, "invalid filter type")
	ErrFilterInputTooShort = errors.Register("tally", 3, "filter input length too short")
	ErrInvalidPathLen      = errors.Register("tally", 4, "invalid JSON path length")
	ErrInvalidNumberType   = errors.Register("tally", 5, "invalid number type specified")
	ErrApplyingFilter      = errors.Register("tally", 6, "failed to apply filter")
	ErrConsensusInError    = errors.Register("tally", 7, "consensus in error")
	ErrNoConsensus         = errors.Register("tally", 8, "> 1/3 of reveals do not agree on reveal data")
	ErrNoBasicConsensus    = errors.Register("tally", 9, "> 1/3 of reveals do not agree on (exit_code_success, proxy_pub_keys)")
	// Errors used in tally execution:
	ErrDecodingPaybackAddress  = errors.Register("tally", 10, "failed to decode payback address")
	ErrFindingTallyProgram     = errors.Register("tally", 11, "failed to find tally program")
	ErrDecodingTallyInputs     = errors.Register("tally", 12, "failed to decode tally inputs")
	ErrConstructingTallyVMArgs = errors.Register("tally", 13, "failed to construct tally VM arguments")
	ErrGettingMaxTallyGasLimit = errors.Register("tally", 14, "failed to get max tally gas limit")
)
