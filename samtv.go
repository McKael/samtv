// Copyright © 2018 Mikael Berthe <mikael@lilotux.net>
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package samtv

import (
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// SmartViewSession contains data for a Smart View session
type SmartViewSession struct {
	tvAddress  string // TV network IP:port
	uuid       string // Device Identifier
	sessionKey []byte // Session encryption key
	sessionID  int    // Session ID

	ws struct {
		c    *websocket.Conn // Websocket Connection
		read chan string     // Websocket message reader
		//write chan string     // Websocket message writer
		state int
		mux   sync.Mutex
	}
}

// Values for the SmartViewSession.ws.state field
const (
	stateNotConnected = iota
	stateOpeningSocket
	stateHandshakeSent
	stateConnected
)

const defaultSessionUUID = "samtv"

// NewSmartViewSession initializes en new SmartViewSession
func NewSmartViewSession(tvAddress string) (SmartViewSession, error) {

	if tvAddress == "" {
		return SmartViewSession{}, errors.New("empty TV IP address")
	}

	// Basic check
	if strings.ContainsRune(tvAddress, ':') {
		return SmartViewSession{}, errors.New("the address should not contain a semicolon")
	}

	svs := SmartViewSession{
		tvAddress: tvAddress,
		uuid:      defaultSessionUUID,
	}

	svs.ws.read = make(chan string, 16)

	return svs, nil
}

// RestoreSessionData sets SmartViewSession key, ID and UUID values
func (s *SmartViewSession) RestoreSessionData(sessionKey []byte, sessionID int, uuid string) {
	if sessionKey != nil {
		s.sessionKey = sessionKey
	}
	if sessionID > 0 {
		s.sessionID = sessionID
	}

	if uuid != "" {
		// XXX Should we check the string?
		s.uuid = uuid
	}
}

// InitSession initiates a websocket connection for the SmartViewSession
func (s *SmartViewSession) InitSession() error {
	if s.tvAddress == "" {
		// Should not happen if NewSmartViewSession has been called before.
		return errors.New("internal error: invalid session")
	}

	// Is connection initiated?
	s.ws.mux.Lock()
	if s.ws.c != nil || s.ws.state != stateNotConnected {
		s.ws.mux.Unlock()
		logrus.Info("InitSession called but a connection is already open")
		return nil
	}
	s.ws.mux.Unlock()

	if err := s.openWSConnection(); err != nil {
		return errors.Wrap(err, "cannot initiate connection")
	}

	<-s.ws.read // Wait for connection

	// We need to pair with the TV if we don't have a session yet
	if len(s.sessionKey) != 16 || s.sessionID <= 0 {
		// No previous session; we need to pair with the Smart TV
		if _, _, _, err := s.Pair(0); err != nil {
			return errors.Wrap(err, "pairing failed")
		}
		logrus.Info("Please use 'samtvcli pair --pin PIN' to associate with the TV")
		return errors.New("pairing required")
	}

	return nil
}

// GetMessage returns the next message received from the device
// If block is true, the read will block for 5 seconds.
func (s *SmartViewSession) GetMessage(block bool) string {
	delay := time.Millisecond
	if block {
		delay = time.Second * 5
	}
	select {
	case msg := <-s.ws.read:
		return msg
	case <-time.After(delay):
		return ""
	}
}

func fetchURL(url string) (string, error) {
	logrus.Debug("Fetch URL: ", url)
	resp, err := http.Get(url)
	if err != nil {
		return "", errors.Wrap(err, "could not send request")
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Wrap(err, "could not read device response")
	}
	return string(body), nil
}
