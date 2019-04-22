package cmd

import (
	"encoding/hex"
	//"time"

	"github.com/pkg/errors"

	"github.com/McKael/samtv"
)

// initSession creates a new SmartViewSession and initialies the connection
func initSession() (*samtv.SmartViewSession, error) {
	// TODO: pre-check server

	s, err := samtv.NewSmartViewSession(server)
	if err != nil {
		return nil, err
	}

	// Pre-check session key (16 hex values)
	// Hex conversion will be done later, so the pre-check only verifies
	// there are 32 chars.
	if l := len(smartSessionKey); l != 0 && l != 32 {
		return nil, errors.New("invalid session key, should be a 32-byte hex string")
	}

	// Use existing session
	sessionKey, err := hex.DecodeString(smartSessionKey)
	if err != nil {
		return nil, errors.Wrap(err, "cannot convert hex key string")
	}

	s.RestoreSessionData(sessionKey, smartSessionID, smartDeviceID)

	err = s.InitSession()
	return s, err
}
