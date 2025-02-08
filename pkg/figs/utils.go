package figs

import "fmt"

func Stringify[T fmt.Stringer](str []T) []string {
	s := make([]string, len(str))
	for i := range str {
		s[i] = str[i].String()
	}
	return s
}
