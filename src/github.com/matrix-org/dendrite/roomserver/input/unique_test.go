package input

import (
	"testing"
)

type sortBytes []byte

func (s sortBytes) Len() int           { return len(s) }
func (s sortBytes) Less(i, j int) bool { return s[i] < s[j] }
func (s sortBytes) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func TestUnique(t *testing.T) {
	testCases := []struct {
		Input string
		Want  string
	}{
		{"aaabbbccc", "abc"},
	}

	for _, test := range testCases {
		input := []byte(test.Input)
		want := string(test.Want)
		got := string(input[:unique(sortBytes(input))])
		if got != want {
			t.Fatal("Wanted ", want, " got ", got)
		}
	}
}
