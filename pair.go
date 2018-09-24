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
	"encoding/hex"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/McKael/smartcrypto"
)

// smartAuthData contains Samsung's Smart TV auth_data response format
type smartAuthData struct {
	AuthType     string  `json:"auth_type"`
	RequestID    string  `json:"request_id"`
	SessionID    *string `json:"session_id"`
	ClientHello  *string `json:"GeneratorClientHello"`
	ClientAckMsg *string `json:"ClientAckMsg"`
}

// Pair handles pairing with the TV device
// If the pin is 0, the PIN popup is requested.
// It returns (deviceid, sessionid, key, error).
func (s *SmartViewSession) Pair(pin int) (string, int, string, error) {
	if pin < 0 {
		return "", 0, "", s.closePINPage()
	}
	if pin == 0 {
		return "", 0, "", s.startPairing()
	}

	// A PIN code was provided.  Process with pairing...

	// _, err := s.pairingExternalStep(1, pin, "")
	if err := s.pairingSteps(pin); err != nil {
		return "", 0, "", err
	}

	// Done -- let's close PIN page
	if err := s.closePINPage(); err != nil {
		logrus.Info("Could not close PIN page: ", err)
	}

	return s.uuid, s.sessionID, hex.EncodeToString(s.sessionKey), nil
}

func (s *SmartViewSession) getTVPairingStepURL(step int) string {
	const appID = "samtvcli"
	return "http://" + s.tvAddress + ":8080/ws/pairing?step=" + strconv.Itoa(step) +
		"&app_id=" + appID + "&device_id=" + s.uuid + "&type=1"
}

func (s *SmartViewSession) startPairing() error {
	if s == nil || s.uuid == "" || s.tvAddress == "" {
		return errors.New("SmartViewSession not initialized")
	}

	logrus.Debugf("Initiating step #0")

	st, err := s.checkPINPage()
	if err != nil {
		st = "stopped"
		logrus.Info("Could not fetch PIN page status: ", err)
	}
	logrus.Debugf("PIN page is %s", st)
	if st != "running" {
		logrus.Info("Requesting PIN page popup...")
		if err := s.openPINPage(); err != nil {
			return errors.Wrap(err, "could not open PIN page")
		}
	}

	step0URL := s.getTVPairingStepURL(0)

	r, err := fetchURL(step0URL)
	if err != nil {
		return errors.Wrap(err, "pairing request failed")
	}
	logrus.Debugf("Pairing request response: `%s`", r)
	return nil
}

func (s *SmartViewSession) openPINPage() error {
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

	return nil
}

func (s *SmartViewSession) closePINPage() error {
	pinClosePageURL := "http://" + s.tvAddress + ":8080/ws/apps/CloudPINPage/run"
	client := &http.Client{}
	req, err := http.NewRequest("DELETE", pinClosePageURL, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (s *SmartViewSession) checkPINPage() (string, error) {
	pinPageURL := "http://" + s.tvAddress + ":8080/ws/apps/CloudPINPage"
	resp, err := http.Get(pinPageURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Wrap(err, "could not read device response")
	}
	body := string(bodyBytes)
	// Basic check
	if !strings.Contains(body, "<name>CloudPINPage</name>") {
		return "", errors.New("unexpected response contents")
	}
	// Get status
	statusRe := regexp.MustCompile("<state>([^<]+)</state>")
	m := statusRe.FindStringSubmatch(body)
	if len(m) < 2 {
		return "", errors.Wrap(err, "could not parse device response")
	}
	return m[1], nil
}

func (s *SmartViewSession) postTVPairingStep(step int, data string) (string, error) {
	stepURL := s.getTVPairingStepURL(step)
	resp, err := http.Post(stepURL, "application/json", bytes.NewBufferString(data))
	if err != nil {
		return "", errors.Wrap(err, "failed to send data to the TV")
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Wrap(err, "could not read device response")
	}
	logrus.Debugf("Step #%d response: `%s`", step, body)

	var response = struct {
		AuthData string `json:"auth_data"`
	}{}
	err = json.Unmarshal([]byte(body), &response)
	if err != nil {
		return "", errors.Wrap(err, "could not decode TV response")
	}
	return response.AuthData, nil
}

func (s *SmartViewSession) pairingSteps(pin int) error {
	const userID = "654321"

	handshake := smartcrypto.HelloData{
		UserID: userID,
		PIN:    strconv.Itoa(pin),
	}

	// Step #1 - Exchange Hello
	logrus.Debugf("Starting pairing step #1 (hello exchange)")

	serverHello, err := smartcrypto.GenerateServerHello(&handshake)
	if err != nil {
		return errors.Wrap(err, "could not generate ServerHello")
	}

	sh := hex.EncodeToString(serverHello)

	logrus.Debugf("Server AES key: %v", handshake.Key)
	logrus.Debugf("Server ctx hash: %v", handshake.Ctx)
	logrus.Debugf("ServerHello: %s", sh)

	content := `{"auth_data":{"auth_type":"SPC","GeneratorServerHello":"` + sh + `"}}`

	body, err := s.postTVPairingStep(1, content)
	if err != nil {
		return errors.Wrap(err, "post step #1")
	}

	logrus.Debugf("Step #1 response body: `%s`", body)

	var step1Response smartAuthData
	err = json.Unmarshal([]byte(body), &step1Response)
	if err != nil {
		return errors.Wrap(err, "step1 failed")
	}
	if step1Response.ClientHello == nil {
		return errors.Wrap(err, "could not get TV ClientHello")
	}

	lastRequestID := step1Response.RequestID

	logrus.Debugf("Request id: `%s`", step1Response.RequestID)
	logrus.Debugf("Client hello: `%s`", *step1Response.ClientHello)

	// Check client hello

	skprime, ctx, err := smartcrypto.ParseClientHello(handshake, *step1Response.ClientHello)
	if err != nil {
		return errors.Wrap(err, "could not parse TV ClientHello")
	}
	logrus.Debugf("SKPrime: `%v`", skprime)
	logrus.Debugf("ctx: `%v`", ctx)

	// Step #2 - Acknowledge Exchange
	logrus.Debugf("Starting pairing step #2 (acknowledge exchange)")

	serverAck, err := smartcrypto.GenerateServerAcknowledge(skprime)
	if err != nil {
		return errors.Wrap(err, "failed to generate server acknowledge")
	}

	content = `{"auth_data":{"auth_type":"SPC","request_id":"` + lastRequestID + `","ServerAckMsg":"` + serverAck + `"}}`
	logrus.Debugf("Step #2 content: %s", content)

	body, err = s.postTVPairingStep(2, content)
	if err != nil {
		return errors.Wrap(err, "post step #2")
	}

	logrus.Debugf("Step #2 response body: `%s`", body)
	var step2Response smartAuthData
	err = json.Unmarshal([]byte(body), &step2Response)
	if err != nil {
		return errors.Wrap(err, "step2 failed")
	}

	if step2Response.SessionID == nil {
		return errors.Wrap(err, "could not get the session ID")
	}
	if step2Response.ClientAckMsg == nil {
		return errors.Wrap(err, "could not get TV ClientAcknowledge")
	}

	clientAck := *step2Response.ClientAckMsg
	sessionID := *step2Response.SessionID
	sid, err := strconv.Atoi(sessionID)
	if err != nil {
		return errors.Wrap(err, "step3: cannot convert session ID to number")
	}

	if err := smartcrypto.ParseClientAcknowledge(clientAck, skprime); err != nil {
		return errors.Wrap(err, "step3: client ack validation failed")
	}
	logrus.Debugf("Client acknowledge is valid")

	logrus.Info("Pairing successful!")

	s.RestoreSessionData([]byte(ctx), sid, "")

	return nil
}
