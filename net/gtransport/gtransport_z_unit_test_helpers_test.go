package gtransport

import (
	"bytes"
	"io"
	"sync"
)

type scriptedManagedRead struct {
	data []byte
	err  error
}

type scriptedManagedWrite struct {
	n   int
	err error
}

type scriptedManagedConn struct {
	mu         sync.Mutex
	reads      []scriptedManagedRead
	writeSteps []scriptedManagedWrite
	writes     [][]byte
	closed     bool
}

func (c *scriptedManagedConn) Read(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return 0, io.EOF
	}
	if len(c.reads) == 0 {
		return 0, io.EOF
	}
	step := c.reads[0]
	c.reads = c.reads[1:]
	n := copy(p, step.data)
	return n, step.err
}

func (c *scriptedManagedConn) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return 0, io.ErrClosedPipe
	}
	if len(c.writeSteps) > 0 {
		step := c.writeSteps[0]
		c.writeSteps = c.writeSteps[1:]
		n := step.n
		if n <= 0 || n > len(p) {
			n = len(p)
		}
		c.writes = append(c.writes, bytes.Clone(p[:n]))
		return n, step.err
	}
	c.writes = append(c.writes, bytes.Clone(p))
	return len(p), nil
}

func (c *scriptedManagedConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return nil
}

func (c *scriptedManagedConn) writePayloads() [][]byte {
	c.mu.Lock()
	defer c.mu.Unlock()

	out := make([][]byte, 0, len(c.writes))
	for _, payload := range c.writes {
		out = append(out, bytes.Clone(payload))
	}
	return out
}
