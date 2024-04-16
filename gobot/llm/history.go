package llm

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

func (h *History) Add(element string) {
	h.data[h.pos] = element
	h.pos = (h.pos + 1) % h.size
}

func (h *History) String() string {
	sb := strings.Builder{}
	sb.Grow(h.size * (len(h.data[0]) + 1))

	idx := h.pos
	for i := 0; i < h.size; i++ {
		sb.WriteString(h.data[idx])
		if i < h.size-1 {
			sb.WriteByte('\n')
		}
		idx = (idx + 1) % h.size
	}

	return sb.String()
}
