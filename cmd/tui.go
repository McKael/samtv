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
	"bufio"
	"fmt"
	"io"
	"os"

	"github.com/jroimartin/gocui"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"mikael/samtv"
)

var logHeight = 7 // Log window height

var tuiLogFile *string

// tuiCmd represents the tui command
var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Text User Interface",
	Long:  `This command runs a curses text user interface.`,
	Run: func(cmd *cobra.Command, args []string) {
		samtvSession, err := initSession()
		if err != nil {
			logrus.Error("Cannot initialize session: ", err)
			os.Exit(1)
		}

		tui(samtvSession)
		samtvSession.Close()
	},
}

func init() {
	RootCmd.AddCommand(tuiCmd)

	tuiLogFile = tuiCmd.Flags().String("log-file", "", "Write logs to file")
}

func tui(samtvSession *samtv.SmartViewSession) {
	if *tuiLogFile != "" {
		if f, err := os.OpenFile(*tuiLogFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600); err != nil {
			logrus.Fatal("Could not open log file: ", err)
		} else {
			logrus.Infof("Redirecting messages to log file '%s'", *tuiLogFile)
			logrus.SetOutput(f)
			defer f.Close()
		}
	}

	g, err := gocui.NewGui(gocui.OutputNormal)
	if err != nil {
		logrus.Fatal(err)
	}
	defer g.Close()

	maxX, maxY := g.Size()

	// For now, don't allow small windows
	if maxX < 80 || maxY < 23 {
		logrus.Fatalln("ERROR: Window size is too small: minimum size is 80x23")
	}

	g.SetManagerFunc(layout)
	g.InputEsc = true

	// Catch ctrl-c
	if err := g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, uiQuit); err != nil {
		logrus.Panicln(err)
	}

	if err := setupKeyBindings(g, samtvSession, tuiDefaultBindings); err != nil {
		logrus.Panicln("Cannot setup key bindings: ", err)
	}

	err = g.MainLoop()
	logrus.SetOutput(os.Stderr) // Restore logging output
	if err != nil && err != gocui.ErrQuit {
		logrus.Fatal(err)
	}

	return
}

func layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()

	logHeight = maxY / 3

	// Main window
	if v, err := g.SetView("main", 0, 0, maxX-1, maxY-logHeight-2); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}

		v.Title = "SamTVcli TUI"
		v.Wrap = true
		v.Autoscroll = true
		// v.Editable = false
		// v.SelFgColor = gocui.ColorYellow
		g.SetCurrentView("main")

		//fmt.Fprintf(v, ...)
	}

	// Log window
	if v, err := g.SetView("log", 0, maxY-logHeight-1, maxX-1, maxY-1); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}

		v.Title = "Log"
		v.Wrap = true
		v.Autoscroll = true

		if *tuiLogFile == "" {
			// We need to pass logs through gocui.Update in
			// order to avoid race conditions.
			r, w := io.Pipe()
			logrus.SetOutput(w)
			go func() {
				scanner := bufio.NewScanner(r)
				for scanner.Scan() {
					text := scanner.Text()
					g.Update(func(*gocui.Gui) error {
						_, e := fmt.Fprintln(v, text)
						return e
					})
				}
			}()
		}
	}

	return nil
}

func setupKeyBindings(g *gocui.Gui, samtvs *samtv.SmartViewSession, keyBindings map[rune]string) error {
	var errs []error

	errs = append(errs, g.SetKeybinding("", gocui.KeyCtrlQ, gocui.ModNone, uiQuit))

	// XXX hardcoded
	errs = append(errs, g.SetKeybinding("main", gocui.KeyEnter, gocui.ModNone,
		genKeyhandler(samtvs, "KEY_ENTER")))
	errs = append(errs, g.SetKeybinding("main", gocui.KeyBackspace, gocui.ModNone,
		genKeyhandler(samtvs, "KEY_RETURN")))
	errs = append(errs, g.SetKeybinding("main", gocui.KeyBackspace2, gocui.ModNone,
		genKeyhandler(samtvs, "KEY_RETURN")))
	errs = append(errs, g.SetKeybinding("main", gocui.KeySpace, gocui.ModNone,
		genKeyhandler(samtvs, "KEY_PLAY")))
	errs = append(errs, g.SetKeybinding("main", gocui.KeyArrowLeft, gocui.ModNone,
		genKeyhandler(samtvs, "KEY_LEFT")))
	errs = append(errs, g.SetKeybinding("main", gocui.KeyArrowDown, gocui.ModNone,
		genKeyhandler(samtvs, "KEY_DOWN")))
	errs = append(errs, g.SetKeybinding("main", gocui.KeyArrowUp, gocui.ModNone,
		genKeyhandler(samtvs, "KEY_UP")))
	errs = append(errs, g.SetKeybinding("main", gocui.KeyArrowRight, gocui.ModNone,
		genKeyhandler(samtvs, "KEY_RIGHT")))

	errs = append(errs, g.SetKeybinding("main", gocui.KeyCtrlSlash, gocui.ModNone, uiToggleDebug))

	for k, code := range keyBindings { // FIXME check nil
		errs = append(errs, g.SetKeybinding("main", k, gocui.ModNone, genKeyhandler(samtvs, code)))
	}

	for _, err := range errs {
		if err != nil {
			return err
		}
	}

	return nil
}

func uiQuit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}

func printLog(g *gocui.Gui, format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	if g == nil {
		fmt.Println(message)
		return
	}
	logView, err := g.View("log")
	if err != nil {
		fmt.Println(message)
		fmt.Println(err)
		return
	}
	fmt.Fprintf(logView, "%s\n", message)
}

func uiToggleDebug(g *gocui.Gui, v *gocui.View) error {
	if debug {
		logrus.SetLevel(logrus.InfoLevel)
	} else {
		logrus.SetLevel(logrus.DebugLevel)
	}
	debug = !debug
	printLog(g, "> Debug: %v", debug)
	return nil
}

func genKeyhandler(s *samtv.SmartViewSession, keyID string) func(*gocui.Gui, *gocui.View) error {
	return func(g *gocui.Gui, v *gocui.View) error {
		printLog(g, "> Send Key %s", keyID)
		if err := s.Key(keyID); err != nil {
			logrus.Error("Cannot send key: ", err)
		}
		return nil
	}
}
