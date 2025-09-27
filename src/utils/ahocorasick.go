// Copyright (c) 2013 CloudFlare, Inc.
//
// Originally released under the BSD 3-Clause license

package utils

import (
	"container/list"
)

type node struct {
	root   bool
	b      []byte
	output bool
	child  [256]*node
	fails  [256]*node
	suffix *node
	fail   *node
}

type Matcher struct {
	trie   []node
	extent int
	root   *node
}

func (m *Matcher) findBlice(b []byte) *node {
	n := &m.trie[0]
	for n != nil && len(b) > 0 {
		n = n.child[int(b[0])]
		b = b[1:]
	}
	return n
}

func (m *Matcher) getFreeNode() *node {
	m.extent += 1
	if m.extent == 1 {
		m.root = &m.trie[0]
		m.root.root = true
	}
	return &m.trie[m.extent-1]
}

func (m *Matcher) buildTrie(dictionary [][]byte) {
	max := 1
	for _, blice := range dictionary {
		max += len(blice)
	}
	m.trie = make([]node, max)
	m.getFreeNode()
	for _, blice := range dictionary {
		n := m.root
		var path []byte
		for _, b := range blice {
			path = append(path, b)

			c := n.child[int(b)]

			if c == nil {
				c = m.getFreeNode()
				n.child[int(b)] = c
				c.b = make([]byte, len(path))
				copy(c.b, path)
				if len(path) == 1 {
					c.fail = m.root
				}

				c.suffix = m.root
			}

			n = c
		}
		n.output = true
	}
	l := new(list.List)
	l.PushBack(m.root)
	for l.Len() > 0 {
		n := l.Remove(l.Front()).(*node)
		for i := range 256 {
			c := n.child[i]
			if c != nil {
				l.PushBack(c)
				for j := 1; j < len(c.b); j++ {
					c.fail = m.findBlice(c.b[j:])
					if c.fail != nil {
						break
					}
				}
				if c.fail == nil {
					c.fail = m.root
				}
				for j := 1; j < len(c.b); j++ {
					s := m.findBlice(c.b[j:])
					if s != nil && s.output {
						c.suffix = s
						break
					}
				}
			}
		}
	}
	for i := 0; i < m.extent; i++ {
		for c := range 256 {
			n := &m.trie[i]
			for n.child[c] == nil && !n.root {
				n = n.fail
			}

			m.trie[i].fails[c] = n
		}
	}
	m.trie = m.trie[:m.extent]
}

func NewMatcher(dictionary [][]byte) *Matcher {
	m := new(Matcher)
	m.buildTrie(dictionary)
	return m
}

func NewStringMatcher(dictionary []string) *Matcher {
	m := new(Matcher)
	var d [][]byte
	for _, s := range dictionary {
		d = append(d, []byte(s))
	}
	m.buildTrie(d)
	return m
}

func (m *Matcher) Contains(in []byte) bool {
	n := m.root
	for _, b := range in {
		c := int(b)
		if !n.root {
			n = n.fails[c]
		}
		if n.child[c] != nil {
			f := n.child[c]
			n = f
			if f.output {
				return true
			}
			if !f.suffix.root {
				return true
			}
		}
	}
	return false
}

func (m *Matcher) Index(in []byte) int {
	n := m.root
	for i, b := range in {
		c := int(b)
		if !n.root {
			n = n.fails[c]
		}
		if n.child[c] != nil {
			f := n.child[c]
			n = f
			if f.output {
				return i - len(f.b) + 1
			}
			for !f.suffix.root {
				f = f.suffix
				if f.output {
					return i - len(f.b) + 1
				}
			}
		}
	}
	return -1
}
