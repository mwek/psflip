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
		"env":      os.Getenv,
		"escapere": regexp.QuoteMeta,
		"cat":      cat,
		"now":      now,
		"utcnow":   utcnow,
	},
)

func cat(path string) string {
	c, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(c)
}

func now(layout string) string {
	return timefmt.Format(time.Now(), layout)
}

func utcnow(layout string) string {
	return timefmt.Format(time.Now().UTC(), layout)
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

// TRegexp is a Regexp with template substitution on unmarshalling
type TRegexp struct {
	*regexp.Regexp
}

// UnmarshalString implements fig.StringUnmarshaler.
func (t *TRegexp) UnmarshalString(str string) error {
	str, err := substitute(str)
	if err != nil {
		return err
	}
	regexp, err := regexp.Compile(str)
	if err != nil {
		return err
	}
	*t = TRegexp{regexp}
	return nil
}

var _ fig.StringUnmarshaler = &TRegexp{}

// TString is a string with template substitution on unmarshalling
type TString struct {
	s string
}

func (s *TString) UnmarshalString(str string) error {
	str, err := substitute(str)
	if err != nil {
		return err
	}
	*s = TString{str}
	return nil
}

func (s TString) String() string {
	return string(s.s)
}

var _ fig.StringUnmarshaler = &TString{}
var _ fmt.Stringer = TString{}
