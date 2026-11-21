package prompt

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func TestSelection(t *testing.T) {
	cases := []struct {
		answer string
		want   []int
		fails  bool
	}{
		{"", []int{1, 2}, false},
		{"all", []int{0, 1, 2, 3, 4}, false},
		{"none", []int{}, false},
		{"5, 1-3", []int{0, 1, 2, 4}, false},
		{"2 2", []int{1}, false},
		{"6", nil, true},
		{"3-1", nil, true},
		{"x", nil, true},
	}

	for _, test := range cases {
		got, err := Selection(test.answer, 5, []int{1, 2})

		if (err != nil) != test.fails || (!test.fails && !reflect.DeepEqual(got, test.want)) {
			t.Errorf("Selection(%q) = %v, %v; want %v (fails %v)", test.answer, got, err, test.want, test.fails)
		}
	}
}

func TestAskAndConfirm(t *testing.T) {
	var out bytes.Buffer
	prompter := New(strings.NewReader("\nShop\nn\n"), &out)

	if answer, _ := prompter.Ask("Name", "fallback"); answer != "fallback" {
		t.Errorf("empty answer: %q", answer)
	}

	if answer, _ := prompter.Ask("Name", "fallback"); answer != "Shop" {
		t.Errorf("answer: %q", answer)
	}

	if confirmed, _ := prompter.Confirm("Send?", true); confirmed {
		t.Error("n confirmed")
	}

	if !strings.Contains(out.String(), "Name [fallback]: ") {
		t.Errorf("question: %q", out.String())
	}
}
