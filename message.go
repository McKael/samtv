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
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type commMessage struct {
	Name string
	Args interface{}
	/* struct {
		SessionID int	`json:"Session_Id"`
		Body string	`json:"body"`
	}
	*/
}

func (s *SmartViewSession) buildMessage(body string) string {
	// TODO build JSON string properly
	return smartMessageCommPrefix + `{"name":"callCommon","args":[{"Session_Id":` +
		fmt.Sprintf("%d", s.sessionID) + `,"body":"[` + body + `]"}]}`
}

func (s *SmartViewSession) parseSmartMessage(msg string) (string, error) {
	if !strings.HasPrefix(msg, smartMessageCommPrefix) {
		return "", errors.New("cannot parse message: unknown prefix")
	}

	msg = msg[len(smartMessageCommPrefix):]

	var res commMessage
	if err := json.Unmarshal([]byte(msg), &res); err != nil {
		return "", errors.Wrap(err, "cannot parse JSON reply")
	}

	if res.Name != "receiveCommon" {
		logrus.Info("msg.Name: ", res.Name) // DBG
	}

	cipherstring, ok := res.Args.(string)
	if !ok {
		logrus.Debug("Could not parse encrypted response: expected list of bytes")
		logrus.Debug("msg.args: ", res.Args)
		return "", errors.New("unhandled args format")
	}

	var cipherdata []byte

	if err := json.Unmarshal([]byte(cipherstring), &cipherdata); err != nil {
		return "", errors.Wrap(err, "cannot parse encrypted response")
	}

	r, err := s.aesDecrypt(cipherdata)
	if err != nil {
		return "", errors.Wrap(err, "cannot decrypt response")
	}
	logrus.Debug("Successfully decrypted response: ", r)
	return string(r), nil
}
