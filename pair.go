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
	"net/url"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// XXX
// (uuid, sid, key, error)
func (s *SmartViewSession) Pair(pin int) (string, int, string, error) {
	if pin <= 0 {
		logrus.Info("Requesting PIN popup...")
		return "", 0, "", s.pairingPopup()
	}

	result, err := s.pairingExternalStep(1, pin, "")
	if err != nil {
		return "", 0, "", err
	}

	logrus.Info(result) // DBG

	// Done -- let's close PIN page
	pinClosePageURL := "http://" + s.tvAddress + ":8080/ws/apps/CloudPINPage/run"
	client := &http.Client{}
	req, err := http.NewRequest("DELETE", pinClosePageURL, nil)
	if err != nil {
		logrus.Info(err) // DBG level
	}
	if resp, err := client.Do(req); err != nil {
		logrus.Info("could not close PIN page: ", err)
	} else {
		resp.Body.Close()
	}

	return s.uuid, s.sessionID, result[0:32], nil
}

func (s *SmartViewSession) pairingPopup() error {
	if s == nil || s.uuid == "" || s.tvAddress == "" {
		return errors.New("SmartViewSession not initialized")
	}

	logrus.Debugf("Initiating step #0")

	pinPageURL := "http://" + s.tvAddress + ":8080/ws/apps/CloudPINPage"
	resp, err := http.PostForm(pinPageURL, url.Values{"data": {"pin4"}})
	if err != nil {
		return errors.Wrap(err, "could not request popup")
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrap(err, "could not read device response")
	}
	logrus.Debugf("PIN page response: `%s`", body)

	step0URL := "http://" + s.tvAddress + ":8080/ws/pairing?step=0" +
		"&app_id=com.samsung.companion&device_id=" + s.uuid +
		"&type=1"

	r, err := fetchURL(step0URL)
	if err != nil {
		return errors.Wrap(err, "pairing failed")
	}
	logrus.Debugf("Pairing request response: `%s`", r)
	return nil
}
