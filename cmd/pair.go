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

package cmd

import (
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/McKael/samtv"
)

var pairingPIN *int

// pairCmd represents the events command
var pairCmd = &cobra.Command{
	Use:   "pair",
	Short: "Pair with a Smart TV",
	Long:  `This command can be used to manage pairing with a Samsung TV.`,
	Example: `  samtvcli pair              # Start pairing process
  samtvcli pair --pin 1234   # Enter TV PIN code
  samtvcli pair --pin -1     # A negative value closes the PIN page`,
	Run: func(cmd *cobra.Command, args []string) {
		s, err := samtv.NewSmartViewSession(server)
		if err != nil {
			logrus.Error(err)
			os.Exit(1)
		}

		// Set UUID
		s.RestoreSessionData(nil, 0, smartDeviceID)

		uuid, sid, key, err := s.Pair(*pairingPIN)
		if err != nil {
			logrus.Error("Pairing error: ", err)
			os.Exit(1)
		}

		if *pairingPIN > 0 && key != "" {
			fmt.Fprintf(os.Stderr, "You can save the following items:\n")
			fmt.Println("device_uuid: ", uuid)
			fmt.Println("session_key: ", key)
			fmt.Println("session_id:  ", sid)
		}
	},
}

func init() {
	RootCmd.AddCommand(pairCmd)

	pairingPIN = pairCmd.Flags().Int("pin", 0, "Pairing PIN code")
}
