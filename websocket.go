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
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	smartMessageInit       = "1::"
	smartMessageHello      = "1::/com.samsung.companion"
	smartMessageKeepalive  = "2::"
	smartMessageCommPrefix = "5::/com.samsung.companion:"

	smartMessageUsualCorrectReply = `"result":{}}`
)

func (s *SmartViewSession) openWSConnection() error {
	const queryPrefix = ":8000/socket.io/1"

	// Open conection
	t := time.Now().UnixNano() / 1000000
	step4URL := "http://" + s.tvAddress + queryPrefix + "/?t=" + strconv.FormatInt(t, 10)
	websocketResponse, err := fetchURL(step4URL)
	if err != nil {
		return errors.Wrap(err, "websocket request failed")
	}

	// Build websocket URL
	wsp := strings.SplitN(websocketResponse, ":", 2)[0]
	u, err := url.Parse("ws://" + s.tvAddress + queryPrefix + "/websocket/" + wsp)
	if err != nil {
		return errors.Wrap(err, "cannot create Websocket URL")
	}

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return errors.Wrap(err, "cannot connect to Websocket")
	}

	// Set up initial read timeout
	c.SetReadDeadline(time.Now().Add(time.Minute))

	s.ws.mux.Lock()
	s.ws.c = c
	s.ws.state = stateOpeningSocket
	s.ws.mux.Unlock()
	go s.manageWS()
	// FIXME stopper

	return nil
}

func (s *SmartViewSession) manageWS() {
	s.ws.mux.Lock()
	if s == nil || s.ws.c == nil {
		s.ws.mux.Unlock()
		logrus.Debugf("manageWS: websocket connection is closed")
		return
	}
	s.ws.mux.Unlock()

LOOP:
	for {
		msg, err := s.readWSMessage()
		s.ws.mux.Lock()
		if s.ws.state == stateNotConnected {
			s.ws.mux.Unlock()
			break LOOP
		}
		s.ws.mux.Unlock()
		if err != nil {
			logrus.Info("socket read failed: ", err)
			s.ws.mux.Lock()
			s.ws.state = stateNotConnected
			s.ws.c.Close()
			s.ws.c = nil
			s.ws.mux.Unlock()
			break LOOP
		}

		switch {
		case msg == smartMessageInit:
			logrus.Debugf("Got greetings from TV")
			s.ws.mux.Lock()
			if s.ws.state != stateOpeningSocket {
				logrus.Debugf("Got init websocket message but current state is %d", s.ws.state)
			}
			s.ws.mux.Unlock()
			logrus.Debug("Sending SmartView handshake...")
			if err := s.sendWSMessage(smartMessageHello); err != nil {
				logrus.Error("Could not send websocket handshake: ", err)
				break LOOP
			}
			s.ws.mux.Lock()
			s.ws.state = stateHandshakeSent
			s.ws.mux.Unlock()
		case msg == smartMessageHello:
			logrus.Debug("SmartView handshake completed")
			s.ws.mux.Lock()
			s.ws.state = stateConnected
			s.ws.mux.Unlock()
			s.ws.read <- msg
		case msg == smartMessageKeepalive:
			logrus.Debug("SmartView keepalive message received")
			s.sendWSMessage(smartMessageKeepalive)
			s.ws.c.SetReadDeadline(time.Now().Add(time.Minute))
		case strings.HasPrefix(msg, smartMessageCommPrefix):
			logrus.Debug("SmartView message received")
			if smsg, err := s.parseSmartMessage(msg); err != nil {
				logrus.Error("Could not parse message: ", err)
			} else {
				logrus.Debug("SmartView message: ", smsg)
				s.ws.read <- smsg
			}
		default:
			logrus.Info("SmartView unhandled message: ", msg)
		}
	}
	logrus.Debug("Leaving manageWS loop")
}

// sendWSMessage sends a raw WebSocket message
func (s *SmartViewSession) sendWSMessage(m string) error {
	s.ws.mux.Lock()
	defer s.ws.mux.Unlock()
	if s.ws.c == nil {
		return errors.New("sendWSMessage: no active websocket connection")
	}
	logrus.Debugf("Sending WS message: `%s` ...", m)
	s.ws.c.SetWriteDeadline(time.Now().Add(15 * time.Second))
	return s.ws.c.WriteMessage(websocket.TextMessage, []byte(m))
}

// readWSMessage reads a WebSocket message
// This method is intended to be used by manageWS.
func (s *SmartViewSession) readWSMessage() (string, error) {
	s.ws.mux.Lock()
	if s.ws.c == nil {
		s.ws.mux.Unlock()
		return "", errors.New("readWSMessage: no active websocket connection")
	}
	logrus.Debugf("Reading WS message...")
	c := s.ws.c
	s.ws.mux.Unlock()
	t, p, err := c.ReadMessage()
	if err != nil {
		return "", err
	}
	logrus.Debugf("Read message (type %d): `%s`", t, string(p))
	if t == websocket.TextMessage {
		return string(p), nil
	}
	return "", nil
}

// Close terminates the websocket connection
func (s *SmartViewSession) Close() {
	s.ws.mux.Lock()
	defer s.ws.mux.Unlock()

	if s.ws.c == nil {
		return // Already closed
	}

	logrus.Debug("Closing websocket")

	s.ws.state = stateNotConnected
	s.ws.c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	s.ws.c.Close()
	s.ws.c = nil
}
