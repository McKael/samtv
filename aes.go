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
	"crypto/aes"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func (s *SmartViewSession) aesEncrypt(plaindata []byte) ([]byte, error) {
	//logrus.Debugf("aesEncrypt(%#v) : '%s'", plaindata, string(plaindata))
	//logrus.Debugf("session ID:  %d", s.sessionID)
	//logrus.Debugf("session key: '%x'\n  %v", string(s.sessionKey), s.sessionKey)

	// Create cipher block
	block, err := aes.NewCipher(s.sessionKey)
	if err != nil {
		return nil, err
	}

	bs := block.BlockSize()
	//logrus.Debugf("block size: %d", bs)

	// Add padding
	padding := bs - len(plaindata)%bs
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	//logrus.Debugf("padding: %d byte(s)", padding)
	plaindata = append(plaindata, padtext...)

	// Encrypt
	ciphertext := make([]byte, len(plaindata))
	for cipherrange := ciphertext; len(plaindata) > 0; {
		block.Encrypt(cipherrange, plaindata[:bs])
		plaindata = plaindata[bs:]
		cipherrange = cipherrange[bs:]
	}

	//logrus.Debugf("ciphertext: %#v", ciphertext)
	return ciphertext, nil
}

func (s *SmartViewSession) aesDecrypt(cipherdata []byte) ([]byte, error) {
	//logrus.Debugf("aesdecrypt : %#v", cipherdata)
	//logrus.Debugf("session ID:  %d", s.sessionID)
	//logrus.Debugf("session key: '%x'\n  %v", string(s.sessionKey), s.sessionKey)

	// Create cipher block
	block, err := aes.NewCipher(s.sessionKey)
	if err != nil {
		return nil, err
	}

	bs := block.BlockSize()
	if len(cipherdata)%bs != 0 {
		return nil, errors.New("encrypted text does not have full blocks")
	}

	// Decrypt
	plaintext := make([]byte, len(cipherdata))
	for plainrange := plaintext; len(cipherdata) > 0; {
		block.Decrypt(plaintext, cipherdata[:bs])
		cipherdata = cipherdata[bs:]
		plainrange = plainrange[bs:]
	}

	// Note: there are null bytes _after_ the padding...
	plaintext, err = pkcs7Unpad(bytes.TrimRight(plaintext, "\x00"), bs)
	if err != nil {
		return nil, err
	}
	logrus.Debugf("plain text: %#v", plaintext) // DBG XXX
	return plaintext, nil
}

// pkcs7Unpad returns slice of the original data without padding
func pkcs7Unpad(data []byte, bs int) ([]byte, error) {
	if bs <= 0 {
		return nil, errors.Errorf("invalid block size %d", bs)
	}
	if len(data)%bs != 0 || len(data) == 0 {
		return nil, errors.Errorf("invalid data len %d", len(data))
	}
	padchar := data[len(data)-1]
	padlen := int(padchar)
	logrus.Debugf("data: %02x", data)     // DBG XXX
	logrus.Debugf("padlen: %02x", padlen) // DBG XXX
	if padlen > len(data) || padlen == 0 {
		return nil, errors.New("invalid padding")
	}
	padstart := len(data) - padlen
	for i := 0; i < padlen; i++ {
		if data[padstart+1] != padchar {
			return nil, errors.New("invalid padding char")
		}
	}

	return data[:padstart], nil
}
