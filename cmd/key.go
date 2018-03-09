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

	"mikael/samtv"
)

var keyList *bool

//var keyHold, keyRelease *bool

// keyCmd represents the key command
var keyCmd = &cobra.Command{
	Use:   "key KEY",
	Short: "Send a key to the TV",
	Long: `Send a key code.

The available key identifiers can be displayed using the --list option.`,
	Example: `  samtvcli key --list
  samtvcli key KEY_VOLDOWN
  samtvcli key KEY_MENU
  samtvcli key KEY_DOWN
  samtvcli key KEY_RETURN
  samtvcli key KEY_POWEROFF`,
	Args: func(cmd *cobra.Command, args []string) error {
		if !*keyList && len(args) != 1 {
			// TODO: allow several keys
			return fmt.Errorf("accepts 1 arg, received %d", len(args))
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		if *keyList {
			for _, k := range samtv.GetKeyCodeList() {
				fmt.Printf("- %s\n", k)
			}
			return
		}

		samtvSession, err := initSession()
		if err != nil {
			logrus.Error("Cannot initialize session: ", err)
			os.Exit(1)
		}

		if err := samtvSession.Key(args[0]); err != nil {
			logrus.Error("Cannot send key: ", err)
		}
		samtvSession.Close()
	},
}

func init() {
	RootCmd.AddCommand(keyCmd)

	keyList = keyCmd.Flags().BoolP("list", "l", false, "List keys")
	//keyHold = keyCmd.Flags().Bool("hold", false, "Hold key pressed")
	//keyRelease = keyCmd.Flags().Bool("release", false, "Release previously-hold key")
}
