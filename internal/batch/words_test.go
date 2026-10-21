package batch

import (
	"reflect"
	"testing"
)

func TestSplitWords(t *testing.T) {
	tests := []struct {
		name, input string
		want        []string
		bad         bool
	}{
		{name: "spaces and tabs", input: "echo\t hello   world", want: []string{"echo", "hello", "world"}},
		{name: "quotes", input: `printf '%s\n' "hello world" '' a"b c"d`, want: []string{"printf", `%s\n`, "hello world", "", "ab cd"}},
		{name: "escapes", input: `echo hello\ world \" "a\qb\"c\\d"`, want: []string{"echo", "hello world", `"`, `a\qb"c\d`}},
		{name: "no evaluation", input: `echo '$HOME' '$(touch file)' \#`, want: []string{"echo", "$HOME", "$(touch file)", "#"}},
		{name: "single quote", input: `echo 'a\b"c'`, want: []string{"echo", `a\b"c`}},
		{name: "unclosed single", input: `echo 'bad`, bad: true},
		{name: "unclosed double", input: `echo "bad`, bad: true},
		{name: "trailing escape", input: `echo \`, bad: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := splitWords(test.input)
			if test.bad {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("got %#v, want %#v", got, test.want)
			}
		})
	}
}
