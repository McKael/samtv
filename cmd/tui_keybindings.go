package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
)

var tuiCurrentBindings map[string]string

// This is a default YAML configuration for the key bindings,
// it can serve as an example as well.
const tuiDefaultBindingsYAML = "  \"`\": " + `KEY_PRECH
  "\"": KEY_PAUSE
  "*": KEY_VOLUP
  "+": KEY_CHUP
  "-": KEY_CHDOWN
  "/": KEY_VOLDOWN
  "<": KEY_REWIND
  ">": KEY_FF
  "(": KEY_REWIND_
  ")": KEY_FF_
  "0": KEY_0
  "1": KEY_1
  "2": KEY_2
  "3": KEY_3
  "4": KEY_4
  "5": KEY_5
  "6": KEY_6
  "7": KEY_7
  "8": KEY_8
  "9": KEY_9
  "B": KEY_BLUE
  "G": KEY_GREEN
  "H": KEY_HOME
  "L": KEY_CH_LIST
  "M": KEY_MENU
  "P": KEY_POWER
  "Q": KEY_EXIT
  "R": KEY_RED
  "Y": KEY_YELLOW
  "d": KEY_HDMI
  "g": KEY_GUIDE
  "h": KEY_LEFT
  "i": KEY_INFO
  "j": KEY_DOWN
  "k": KEY_UP
  "l": KEY_RIGHT
  "m": KEY_MUTE
  "p": KEY_STOP
  "s": KEY_SOURCE
  "t": KEY_TV

  "q": TUI_QUIT
`

func tuiLoadKeyBindings(yamlText string) error {
	var b map[string]string

	if err := yaml.Unmarshal([]byte(yamlText), &b); err != nil {
		return err
	}

	tuiCurrentBindings = b
	return nil
}

func tuiListKeyBindings(maxWidth int) string {
	bk := make([]string, len(tuiCurrentBindings))
	maxLen := 0
	i := 0
	for k, a := range tuiCurrentBindings {
		bk[i] = k
		i++
		l := len(k) + len(a)
		if l > maxLen {
			maxLen = l
		}
	}
	sort.Strings(bk)

	ncol := maxWidth / (maxLen + 10)
	switch {
	case ncol < 1:
		ncol = 1
	case ncol > 4:
		ncol = 4
	}
	colW := maxWidth/ncol - 1

	var b strings.Builder
	i = 0
	n := len(bk)
	displayed := 0

	for displayed < n {
		idx := ((n+ncol-1)/ncol)*(i%ncol) + i/ncol
		i++
		if idx >= n {
			if i%ncol == 0 {
				b.WriteString("\n")
			}
			continue
		}
		k := bk[idx]
		a := tuiCurrentBindings[k]
		fmt.Fprintf(&b, " %s -> %s", k, a)
		displayed++
		if i%ncol == 0 {
			b.WriteString("\n")
			continue
		}
		if displayed == n {
			break
		}
		fmt.Fprintf(&b, strings.Repeat(" ", colW-4-len(k)-len(a)))
	}

	return b.String()
}

func getKeyFromString(k string) (interface{}, error) {
	if len(k) == 1 {
		return rune(k[0]), nil
	}
	if k == "" {
		return nil, errors.New("empty key string")
	}
	// TODO: Add special keys here...
	return nil, errors.New("unsupported key identifier")
}
