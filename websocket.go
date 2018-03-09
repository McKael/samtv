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

	s.ws.c = c
	s.ws.state = stateOpeningSocket
	go s.manageWS()
	// FIXME stopper

	return nil
}

func (s *SmartViewSession) manageWS() {
	if s == nil || s.ws.c == nil {
		return
	}

LOOP:
	for {
		msg, err := s.readWSMessage() // XXX timeout?
		s.ws.mux.Lock()
		if s.ws.state == stateNotconnected {
			s.ws.mux.Unlock()
			break LOOP
		}
		s.ws.mux.Unlock()
		if err != nil {
			logrus.Error("socket read failed: ", err)
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
	logrus.Info("[DEBUG] Out of manageWS loop")
}

// sendWSMessage sends a raw WebSocket message
func (s *SmartViewSession) sendWSMessage(m string) error {
	if s.ws.c == nil {
		return errors.New("sendWSMessage: no active websocket connection")
	}
	logrus.Debugf("Sending WS message: `%s` ...", m)
	return s.ws.c.WriteMessage(websocket.TextMessage, []byte(m))
}

// readWSMessage reads a WebSocket message
func (s *SmartViewSession) readWSMessage() (string, error) {
	if s.ws.c == nil {
		return "", errors.New("readWSMessage: no active websocket connection")
	}
	logrus.Debugf("Reading WS message...")
	t, p, err := s.ws.c.ReadMessage()
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
func (s *SmartViewSession) Close() { // TODO error
	if s.ws.c == nil {
		return // Already closed
	}

	logrus.Debug("Closing websocket")

	s.ws.mux.Lock()
	s.ws.state = stateNotconnected
	s.ws.c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	s.ws.c.Close()
	s.ws.c = nil
	s.ws.mux.Unlock()
	// FIXME TODO : stop socket manager
}
