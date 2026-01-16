package steam

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"unicode"
)

type VDFNode map[string]any

func ParseVDF(r io.Reader) (VDFNode, error) {
	scanner := bufio.NewScanner(r)
	root := make(VDFNode)
	stack := []VDFNode{root}
	var currentKey string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		if line == "{" {
			newNode := make(VDFNode)
			stack[len(stack)-1][currentKey] = newNode
			stack = append(stack, newNode)
			currentKey = ""
			continue
		}

		if line == "}" {
			if len(stack) <= 1 {
				return nil, fmt.Errorf("unexpected closing brace")
			}
			stack = stack[:len(stack)-1]
			continue
		}

		tokens := tokenizeLine(line)
		if len(tokens) == 1 {
			currentKey = tokens[0]
		} else if len(tokens) == 2 {
			stack[len(stack)-1][tokens[0]] = tokens[1]
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return root, nil
}

func tokenizeLine(line string) []string {
	var tokens []string
	var current strings.Builder
	inQuotes := false

	for _, r := range line {
		if r == '"' {
			if inQuotes {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			inQuotes = !inQuotes
			continue
		}

		if inQuotes {
			current.WriteRune(r)
		} else if !unicode.IsSpace(r) {
			current.WriteRune(r)
		} else if current.Len() > 0 {
			tokens = append(tokens, current.String())
			current.Reset()
		}
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}

func (n VDFNode) GetNode(key string) VDFNode {
	if v, ok := n[key]; ok {
		if node, ok := v.(VDFNode); ok {
			return node
		}
	}
	return nil
}

func (n VDFNode) GetString(key string) string {
	if v, ok := n[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
