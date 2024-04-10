package main

import "strings"

type History struct {
	data []string
	size int
	pos  int
}

func NewHistory(size int) *History {
	return &History{
		data: make([]string, size),
		size: size,
		pos:  0,
	}
}

func (fs *History) Add(element string) {
	fs.data[fs.pos] = element
	fs.pos = (fs.pos + 1) % fs.size
}

func (fs *History) String() string {
	result := make([]string, fs.size)
	for i := 0; i < fs.size; i++ {
		result[i] = fs.data[(fs.pos+i)%fs.size]
	}
	return strings.Join(result, "\n")
}
