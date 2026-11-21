// Package prompt asks questions in the terminal.
package prompt

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

// Prompter reads answers from one input and writes questions to one output.
type Prompter struct {
	in  *bufio.Reader
	out io.Writer
}

// New is a prompter on the input and output.
func New(in io.Reader, out io.Writer) *Prompter {
	return &Prompter{in: bufio.NewReader(in), out: out}
}

// Ask asks a question; an empty answer is the default.
func (prompter *Prompter) Ask(question, fallback string) (string, error) {
	if fallback != "" {
		fmt.Fprintf(prompter.out, "%s [%s]: ", question, fallback)
	} else {
		fmt.Fprintf(prompter.out, "%s: ", question)
	}

	line, err := prompter.in.ReadString('\n')

	if err != nil && line == "" {
		return "", err
	}

	if answer := strings.TrimSpace(line); answer != "" {
		return answer, nil
	}

	return fallback, nil
}

// Confirm asks a yes/no question.
func (prompter *Prompter) Confirm(question string, fallback bool) (bool, error) {
	hint := "y/N"

	if fallback {
		hint = "Y/n"
	}

	answer, err := prompter.Ask(question+" ("+hint+")", "")

	if err != nil {
		return false, err
	}

	switch strings.ToLower(answer) {
	case "":
		return fallback, nil
	case "y", "yes", "j", "ja":
		return true, nil
	default:
		return false, nil
	}
}

// Selection parses a choice of numbered items (1 … count): "" the defaults, "all", "none", or numbers and ranges
// ("1,3-5"). The result is sorted, each number once, 0-based.
func Selection(answer string, count int, defaults []int) ([]int, error) {
	answer = strings.ToLower(strings.TrimSpace(answer))

	switch answer {
	case "":
		return defaults, nil
	case "all", "a":
		all := make([]int, count)

		for index := range all {
			all[index] = index
		}

		return all, nil
	case "none", "n", "-":
		return []int{}, nil
	}

	chosen := map[int]bool{}

	for _, part := range strings.FieldsFunc(answer, func(r rune) bool { return r == ',' || r == ' ' || r == ';' }) {
		from, to, isRange := strings.Cut(part, "-")
		first, err := strconv.Atoi(strings.TrimSpace(from))

		if err != nil {
			return nil, fmt.Errorf("%q is no number", part)
		}

		last := first

		if isRange {
			if last, err = strconv.Atoi(strings.TrimSpace(to)); err != nil {
				return nil, fmt.Errorf("%q is no range", part)
			}
		}

		if first < 1 || last > count || first > last {
			return nil, fmt.Errorf("%q is not between 1 and %d", part, count)
		}

		for number := first; number <= last; number++ {
			chosen[number-1] = true
		}
	}

	result := make([]int, 0, len(chosen))

	for index := range chosen {
		result = append(result, index)
	}

	sort.Ints(result)

	return result, nil
}
