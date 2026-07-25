package app

import (
	"strings"
	"sync"
)

type acNode struct {
	children map[rune]*acNode
	fail     *acNode
	word     []rune
	end      bool
}

type acMatcher struct {
	root *acNode
}

type acHit struct {
	Pos  int
	Word []rune
}

var (
	identityACMu    sync.Mutex
	identityACCache = map[string]*acMatcher{}
)

func getOrBuildIdentityAC(words []string) *acMatcher {
	key := strings.Join(words, "\x00")
	identityACMu.Lock()
	defer identityACMu.Unlock()
	if matcher, ok := identityACCache[key]; ok {
		return matcher
	}
	matcher := buildIdentityAC(words)
	identityACCache[key] = matcher
	return matcher
}

func buildIdentityAC(words []string) *acMatcher {
	root := &acNode{children: map[rune]*acNode{}}
	for _, word := range words {
		word = lowerString(word)
		if word == "" {
			continue
		}
		node := root
		for _, r := range []rune(word) {
			if node.children[r] == nil {
				node.children[r] = &acNode{children: map[rune]*acNode{}}
			}
			node = node.children[r]
		}
		node.end = true
		node.word = []rune(word)
	}

	queue := make([]*acNode, 0)
	for _, child := range root.children {
		child.fail = root
		queue = append(queue, child)
	}
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		for r, child := range node.children {
			fail := node.fail
			for fail != nil && fail.children[r] == nil {
				fail = fail.fail
			}
			if fail == nil {
				child.fail = root
			} else {
				child.fail = fail.children[r]
			}
			queue = append(queue, child)
		}
	}
	return &acMatcher{root: root}
}

func (m *acMatcher) MultiPatternSearch(text []rune, returnImmediately bool) []acHit {
	if m == nil || m.root == nil {
		return nil
	}
	node := m.root
	hits := make([]acHit, 0)
	for i, r := range text {
		for node != m.root && node.children[r] == nil {
			node = node.fail
		}
		if next := node.children[r]; next != nil {
			node = next
		}
		cursor := node
		for cursor != nil && cursor != m.root {
			if cursor.end {
				hits = append(hits, acHit{
					Pos:  i - len(cursor.word) + 1,
					Word: cursor.word,
				})
				if returnImmediately {
					return hits
				}
			}
			cursor = cursor.fail
		}
	}
	return hits
}

func lowerString(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
