package main

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"time"
)

type TelnetClient interface {
	Connect() error
	io.Closer
	Send() error
	Receive() error
}

func NewTelnetClient(address string, timeout time.Duration, in io.ReadCloser, out io.Writer) TelnetClient {
	return &telnetClient{address: address, timeout: timeout, in: in, out: out}
}

type telnetClient struct {
	address   string
	timeout   time.Duration
	in        io.ReadCloser
	out       io.Writer
	conn      net.Conn
	closeOnce sync.Once
	closeErr  error
}

func (c *telnetClient) Connect() error {
	var err error
	dialer := net.Dialer{Timeout: c.timeout}
	c.conn, err = dialer.DialContext(context.Background(), "tcp", c.address)
	return err
}

func (c *telnetClient) Close() error {
	c.closeOnce.Do(func() {
		if c.conn != nil {
			c.closeErr = c.conn.Close()
		}
		c.closeErr = errors.Join(c.closeErr, c.in.Close())
	})
	return c.closeErr
}

func (c *telnetClient) Send() error {
	if c.conn == nil {
		return net.ErrClosed
	}
	_, err := io.Copy(c.conn, c.in)
	return err
}

func (c *telnetClient) Receive() error {
	if c.conn == nil {
		return net.ErrClosed
	}
	_, err := io.Copy(c.out, c.conn)
	return err
}
