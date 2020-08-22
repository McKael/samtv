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
	"io/ioutil"
	"os"
	"strings"

	"github.com/jroimartin/gocui"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/McKael/samtv"
)

var logHeight = 7 // Log window height

var tuiKeybindingsConfigFile *string
var tuiLogFile *string
var tuiLogWriter *io.PipeWriter

// tuiCmd represents the tui command
var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Text User Interface",
	Long:  `This command runs a curses text user interface.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Load keybindings
		tuiBindingsYAML := tuiDefaultBindingsYAML
		keybindingsFile := viper.GetString("keybindings")
		if keybindingsFile != "" {
			// User-provided bindings
			cfbytes, err := ioutil.ReadFile(keybindingsFile)
			if err != nil {
				logrus.Fatal("Could not read key bindings configuration file: ", err)
			}
			logrus.Debugf("Loading keybindings file '%s'", keybindingsFile)
			tuiBindingsYAML = string(cfbytes)
		}
		if err := tuiLoadKeyBindings(tuiBindingsYAML); err != nil {
			logrus.Fatal("Could not read key bindings configuration: ", err)
		}

		// Start SmartView Session
		samtvSession, err := initSession()
		if err != nil {
			logrus.Error("Cannot initialize session: ", err)
			os.Exit(1)
		}

		// Run TUI
		tui(samtvSession)
		samtvSession.Close()
	},
}

func init() {
	RootCmd.AddCommand(tuiCmd)

	tuiLogFile = tuiCmd.Flags().String("log-file", "", "Write logs to file")
	tuiKeybindingsConfigFile = tuiCmd.Flags().String("keybindings", "",
		"Path to keybindings config file (YAML)")

	viper.BindPFlag("keybindings", tuiCmd.Flags().Lookup("keybindings"))
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
		g.Close()
		logrus.Fatalln("ERROR: Window size is too small: minimum size is 80x23")
	}

	g.SetManagerFunc(layout)
	g.InputEsc = true

	// Catch ctrl-c
	if err := g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, uiQuit); err != nil {
		logrus.Panicln(err)
	}

	if err := setupKeyBindings(g, samtvSession, tuiCurrentBindings); err != nil {
		g.Close()
		logrus.Fatal("Cannot setup key bindings: ", err)
	}

	err = g.MainLoop()
	if tuiLogWriter != nil {
		tuiLogWriter.Close()
	} else {
		logrus.SetOutput(os.Stderr) // Restore logging output
	}

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
		v.Wrap = false
		v.Autoscroll = false
		// v.Editable = false
		// v.SelFgColor = gocui.ColorYellow
		g.SetCurrentView("main")

		helpMessage := `Persistent bindings:
C-q:         Quit samtvcli              PgUp/PgDown:    Scroll up/down
Enter:       KEY_ENTER                  BackSpace:      KEY_RETURN
Left/Right:  KEY_LEFT/RIGHT             Up/Down:        KEY_UP/DOWN
Space:       KEY_PLAY

Dynamic bindings:
` + tuiListKeyBindings(maxX-2)
		fmt.Fprint(v, helpMessage)

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
			tuiLogWriter = w
			go func() {
				scanner := bufio.NewScanner(r)
				for scanner.Scan() {
					text := scanner.Text()
					g.Update(func(*gocui.Gui) error {
						_, e := fmt.Fprintln(v, text)
						return e
					})
				}

				logrus.SetOutput(os.Stderr)              // Restore logging output
				logrus.Debug("Leaving log pipe routine") // DBG
			}()
		}
	}

	return nil
}

func setupKeyBindings(g *gocui.Gui, samtvs *samtv.SmartViewSession, keyBindings map[string]string) error {
	var errs []error

	errs = append(errs,
		g.SetKeybinding("", gocui.KeyCtrlQ, gocui.ModNone, uiQuit),

		g.SetKeybinding("main", gocui.KeyPgup, gocui.ModNone, scrollPageUp),
		g.SetKeybinding("main", gocui.KeyPgdn, gocui.ModNone, scrollPageDown),

		// XXX hardcoded
		g.SetKeybinding("main", gocui.KeyEnter, gocui.ModNone,
			genKeyhandler(samtvs, "KEY_ENTER")),
		g.SetKeybinding("main", gocui.KeyBackspace, gocui.ModNone,
			genKeyhandler(samtvs, "KEY_RETURN")),
		g.SetKeybinding("main", gocui.KeyBackspace2, gocui.ModNone,
			genKeyhandler(samtvs, "KEY_RETURN")),
		g.SetKeybinding("main", gocui.KeySpace, gocui.ModNone,
			genKeyhandler(samtvs, "KEY_PLAY")),
		g.SetKeybinding("main", gocui.KeyArrowLeft, gocui.ModNone,
			genKeyhandler(samtvs, "KEY_LEFT")),
		g.SetKeybinding("main", gocui.KeyArrowDown, gocui.ModNone,
			genKeyhandler(samtvs, "KEY_DOWN")),
		g.SetKeybinding("main", gocui.KeyArrowUp, gocui.ModNone,
			genKeyhandler(samtvs, "KEY_UP")),
		g.SetKeybinding("main", gocui.KeyArrowRight, gocui.ModNone,
			genKeyhandler(samtvs, "KEY_RIGHT")),

		g.SetKeybinding("main", gocui.KeyCtrlSlash, gocui.ModNone, uiToggleDebug),
	)

	if keyBindings != nil {
		for k, code := range keyBindings {
			// Check k validity
			kr, err := getKeyFromString(k)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			errs = append(errs, g.SetKeybinding("main", kr,
				gocui.ModNone, genKeyhandler(samtvs, code)))
		}
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
		if strings.HasPrefix(keyID, "TUI_") {
			return tuiInternalCommand(g, v, keyID)
		}

		if !strings.HasPrefix(keyID, "KEY_") {
			return errors.New("invalid key identifier in shortcut")
		}

		// This is a Samsung key
		printLog(g, "> Send Key %s", keyID)

		go func() {
			var msg string
			if err := s.Key(keyID); err != nil {
				msg = fmt.Sprintf("Failed to send Key %s", keyID)
				logrus.Error("Cannot send key: ", err)
			} else {
				msg = fmt.Sprintf("Key sent successfully (%s)", keyID)
			}
			// Use gocui.Update to display log message since we're
			// in a goroutine
			g.Update(func(*gocui.Gui) error {
				printLog(g, msg)
				return nil
			})
		}()
		return nil
	}
}

func tuiInternalCommand(g *gocui.Gui, v *gocui.View, keyID string) error {
	switch keyID {
	case "TUI_QUIT":
		return uiQuit(g, v)
	}
	return nil
}

func scrollPageUp(g *gocui.Gui, v *gocui.View) error {
	ox, oy := v.Origin()

	if oy < 1 {
		return nil
	}

	_, h := v.Size()
	oy -= h / 2
	if oy < 0 {
		oy = 0
	}

	return v.SetOrigin(ox, oy)
}

func scrollPageDown(g *gocui.Gui, v *gocui.View) error {
	ox, oy := v.Origin()

	_, h := v.Size()
	oy += h / 2
	if oy >= strings.Count(v.Buffer(), "\n") {
		return nil // Ignore
	}

	return v.SetOrigin(ox, oy)
}
