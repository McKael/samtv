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
	"bytes"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func (s *SmartViewSession) pairingExternalStep(step int, pin int, prevStepData string) (string, error) {
	logrus.Debugf("Starting pairing step #%d", step)
	payload := struct {
		Pin      int    `json:"pin"`
		Payload  string `json:"payload"`
		DeviceID string `json:"deviceId"`
	}{Pin: pin, DeviceID: s.uuid}

	payload.Payload = prevStepData

	b, err := json.Marshal(payload)
	if err != nil {
		return "", errors.Wrap(err, "failed to encode data")
	}

	//logrus.Info("Payload: ", string(b)) // DBG

	r, err := pairingServerPost("/step"+strconv.Itoa(step), b)
	if err != nil {
		return "", errors.Wrap(err, "could not query external server")
	}

	if len(r) == 0 {
		return "", errors.New("empty response from external server")
	}

	logrus.Debugf("External server returned: `%s`", r) // DBG

	// Try to parse the result as a JSON message (this is used for server
	// errors...).
	var possibleResult = struct {
		Status  int
		Error   string
		Message string
	}{}
	err = json.Unmarshal([]byte(r), &possibleResult)
	if err == nil && possibleResult.Status != 0 {
		logrus.Infof("External server sent status %d (%s): %s",
			possibleResult.Status, possibleResult.Error, possibleResult.Message)
		return "", errors.Errorf("step%d: unexpected reply from external server: [%s] %s",
			step, possibleResult.Error, possibleResult.Message)
	}

	if step == 3 {
		// We're almost done
		var res = struct {
			SessionKey string `json:"session_key"`
			SessionID  string `json:"session_id"`
		}{}
		if err := json.Unmarshal([]byte(r), &res); err != nil {
			return "", errors.Wrap(err, "step3: cannot parse response")
		}
		if len(res.SessionKey) != 32 {
			return "", errors.Errorf("step3: wrong session key length: %d", len(res.SessionKey))
		}
		sessionKey, err := hex.DecodeString(res.SessionKey)
		if err != nil {
			return "", errors.Wrap(err, "step3: cannot convert hex key string")
		}

		logrus.Info("Pairing successful!")

		sid, err := strconv.Atoi(res.SessionID)
		if err != nil {
			return "", errors.Wrap(err, "step3: cannot convert session ID to number")
		}
		s.RestoreSessionData(sessionKey, sid, "")

		// Return information
		return res.SessionKey + "/" + res.SessionID, nil
	}

	stepURL := "http://" + s.tvAddress + ":8080/ws/pairing?step=" + strconv.Itoa(step) +
		"&app_id=com.samsung.companion&device_id=" + s.uuid +
		"&type=1"
	resp, err := http.Post(stepURL, "application/json", bytes.NewBufferString(r))
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Wrap(err, "could not read device response")
	}
	logrus.Debugf("Step #%d response: `%s`", step, body)

	bodyString := strings.Replace(string(body), `\"`, `"`, -1)
	//logrus.Debugf("... resp stripped 1 to: `%s`", r) // DBG XXX

	// Proceed to next step!
	return s.pairingExternalStep(step+1, pin, bodyString)
}

/* XXX TODO step1 authdata
if (Encryption.parseClientHello(authData.GeneratorClientHello) !== 0) {
    console.error('Invalid PIN Entered')
}
*/
func pairingServerPost(query string, data []byte) (string, error) {
	// External server for crypto,
	// see https://github.com/eclair4151/samsung_encrypted_POC
	const externalPairingServer = "https://34.210.190.209:5443"

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	req, err := http.NewRequest("POST", externalPairingServer+query, bytes.NewBuffer(data))

	req.Header.Add("Authorization", "Basic b3JjaGVzdHJhdG9yOnBhc3N3b3Jk")
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("User-Agent", "CFNetwork/893.7 Darwin/17.3.0")

	//logrus.Debug(req)

	resp, err := client.Do(req)
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
