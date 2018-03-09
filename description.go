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

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// SmartDeviceDescription contains a TV device description
type SmartDeviceDescription struct {
	DUID              string
	Model             string
	ModelName         string
	ModelDescription  string
	NetworkType       string
	SSID              string
	IP                string
	FirmwareVersion   string
	DeviceName        string
	DeviceID          string
	UDN               string
	Resolution        string
	CountryCode       string
	SmartHubAgreement string
	ServiceURI        string
	DialURI           string
	Capabilities      []struct {
		Name     string
		Port     string
		Location string
	}
}

// DeviceDescription fetches the description from the TV device
func (s *SmartViewSession) DeviceDescription() (SmartDeviceDescription, error) {
	var sdd SmartDeviceDescription
	if s.tvAddress == "" {
		// Should not happen if NewSmartViewSession has been called before.
		return sdd, errors.New("internal error: invalid session, missing TV IP address")
	}

	d, err := fetchURL("http://" + s.tvAddress + ":8001/ms/1.0/")
	if err != nil {
		return sdd, err
	}

	if err := json.Unmarshal([]byte(d), &sdd); err != nil {
		logrus.Info(d)
		return sdd, errors.Wrap(err, "cannot parse JSON description")
	}

	return sdd, nil
}
