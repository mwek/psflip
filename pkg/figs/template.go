package figs

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/itchyny/timefmt-go"
	"github.com/kkyr/fig"
)

var t = template.New("psflip").Funcs(
	template.FuncMap{
		"Env":       os.Getenv,
		"AB":        AB,
		"BlueGreen": BlueGreen,
		"EscapeRE":  regexp.QuoteMeta,
		"Cat":       Cat,
		"Now":       Now,
		"UTCNow":    UTCNow,
		"Local":     GetLocal,
	},
)

const (
	abEnv = "PSFLIP_AB_FLAG"
	abA   = "a"
	abB   = "b"
)

var (
	abFlag string
	locals map[string]string
)

func init() {
	if os.Getenv(abEnv) == abA {
		os.Setenv(abEnv, abB)
		abFlag = abB
	} else {
		os.Setenv(abEnv, abA)
		abFlag = abA
	}
}

func SetLocals[T fmt.Stringer](l map[string]T) {
	locals = make(map[string]string, len(l))
	for k, v := range l {
		locals[k] = v.String()
	}
}

func GetLocal(name string) (string, error) {
	if v, ok := locals[name]; ok {
		return v, nil
	} else {
		return "", fmt.Errorf("local %s not found", name)
	}
}

// AB initially returns s1, and then alternates between s1 and s2 on each process upgrade.
func AB(s1, s2 string) (string, error) {
	switch abFlag {
	case abA:
		return s1, nil
	case abB:
		return s2, nil
	default:
		return "", fmt.Errorf("invalid PSFLIP_AB_FLAG value: %s", abFlag)
	}
}

// BlueGreen starts from returning "blue", and then alternates between "blue" and "green" on each process upgrade.
// Alias to {{ AB "blue" "green" }}
func BlueGreen() (string, error) {
	return AB("blue", "green")
}

// Cat returns the content of the file designated by `path`, falling back to empty string on error.
func Cat(path string) (string, error) {
	c, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(c), nil
}

var tStart = time.Now()

// Now returns the start time of the process, formatted with a given `layout` following `strftime`.
func Now(layout string) string {
	return timefmt.Format(tStart, layout)
}

// UTCNow returns the start time of the process in UTC timezone, formatted with a given `layout` following `strftime`.
func UTCNow(layout string) string {
	return timefmt.Format(tStart.UTC(), layout)
}

func substitute(s string) (string, error) {
	templ, err := t.Parse(s)
	if err != nil {
		return "", err
	}
	sb := strings.Builder{}
	err = templ.Execute(&sb, nil)
	if err != nil {
		return "", err
	}
	return sb.String(), nil
}

// TString is a string with template substitution on unmarshalling
type TString string

func (s *TString) UnmarshalString(str string) error {
	str, err := substitute(str)
	if err != nil {
		return err
	}
	*s = TString(str)
	return nil
}

func (s TString) String() string {
	return string(s)
}

// Enforce interface implementation
var _ fmt.Stringer = TString("")
var _ fig.StringUnmarshaler = (*TString)(nil)
