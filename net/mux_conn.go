package net

import (
	"encoding/json"
	"io"
	"math"
	"net"
	"sync"
	"time"

	"github.com/ernestrc/blue/logging"
	multierr "github.com/ernestrc/go-multierror"
	"github.com/sirupsen/logrus"
)

const (
	readChanBuffer   = 1
	writeChanBuffer  = 1
	controlMessageID = 0
	masterConnID     = 1
)

// MuxConn implements a net.Conn capable of multiplexing
// multiple logical connections, bidirectionally, over the
// same underlying net.Conn.
type MuxConn struct {
	root     net.Conn
	nextID   uint16
	quitChan chan struct{}

	wg          sync.WaitGroup
	mu          sync.Mutex
	connections map[uint16]chanConnPair
}

// NewMuxConn allocates storage for a new MuxConn and initializes it.
// The flag host controls whether this side of the connection is the host-side.
// Failure to correctly set this flag will lead to collisions between connections.
func NewMuxConn(over net.Conn, host bool) (*MuxConn, error) {
	ret := new(MuxConn)
	ret.root = over
	ret.quitChan = make(chan struct{})
	ret.connections = make(map[uint16]chanConnPair, 1)
	// control commands is id 0, master is 1
	ret.nextID = masterConnID
	_, _, err := ret.Mux()
	if err != nil {
		return nil, err
	}
	if !host {
		ret.nextID = math.MaxUint16 / 2
	} else {
		ret.nextID = 2
	}
	go ret.read()
	return ret, nil
}

// Dial dials to the other end of this MuxConn for a connection
// with the given id. If the connection does not exist, an error
// is returned on the next call to Read or Write.
func (c *MuxConn) Dial(id uint16) (net.Conn, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// NOTE: send a control msg to test if connection exists
	return c.newChanConn(id), nil
}

func (c *MuxConn) newChanConn(id uint16) net.Conn {
	read := make(chan Result, readChanBuffer)
	write := make(chan Result, writeChanBuffer)
	chanConn := ChanConn(c.root.LocalAddr(), c.root.RemoteAddr(), read, write)
	c.connections[id] = chanConnPair{
		readCh: read,
		conn:   chanConn,
	}
	c.wg.Add(1)
	go c.writeChanConn(id, write)
	return chanConn
}

// Multiplex creates a new net.Conn on this end of the connection
// which can be dialed on the other end of the connection via Dial.
func (c *MuxConn) Mux() (uint16, net.Conn, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	id := c.nextID
	chanConn := c.newChanConn(id)
	c.nextID++
	return id, chanConn, nil
}

// Read satisfies net.Conn.
func (c *MuxConn) Read(b []byte) (n int, err error) {
	master, err := c.master()
	if err != nil {
		return 0, err
	}
	return master.Read(b)
}

// Write satisfies net.Conn.
func (c *MuxConn) Write(b []byte) (n int, err error) {
	master, err := c.master()
	if err != nil {
		return 0, err
	}
	return master.Write(b)
}

// LocalAddr returns the local net.Addr of the underlying connection.
func (c *MuxConn) LocalAddr() net.Addr {
	return c.root.LocalAddr()
}

// RemoteAddr returns the remote net.Addr of the underlying connection.
func (c *MuxConn) RemoteAddr() net.Addr {
	return c.root.RemoteAddr()
}

// SetDeadline sets the read and write deadlines of this net.Conn.
// This operation only applies to the logical net.Conn exposed by
// this MuxConn, not to any of its other multiplexed connection.
func (c *MuxConn) SetDeadline(t time.Time) error {
	master, err := c.master()
	if err != nil {
		return err
	}
	return master.SetDeadline(t)
}

// SetReadDeadline sets the read deadlines of this net.Conn.
// This operation only applies to the logical net.Conn exposed by
// this MuxConn, not to any of its other multiplexed connection.
func (c *MuxConn) SetReadDeadline(t time.Time) error {
	master, err := c.master()
	if err != nil {
		return err
	}
	return master.SetReadDeadline(t)
}

// SetWriteDeadline sets the read deadlines of this net.Conn.
// This operation only applies to the logical net.Conn exposed by
// this MuxConn, not to any of its other multiplexed connection.
func (c *MuxConn) SetWriteDeadline(t time.Time) error {
	master, err := c.master()
	if err != nil {
		return err
	}
	return master.SetWriteDeadline(t)
}

// Close closes all resources associated with this MuxConn.
func (c *MuxConn) Close() (ret error) {
	c.mu.Lock()
	if c.connections == nil {
		c.mu.Unlock()
		return
	}
	temp := c.connections
	c.connections = nil
	c.mu.Unlock()

	for _, conn := range temp {
		if err := conn.conn.Close(); err != nil {
			ret = multierr.Append(ret, err)
		}
	}

	if err := c.root.Close(); err != nil {
		ret = multierr.Append(ret, err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// wait for all writes to be consumed before closing quitChan
	// and force all worker goroutines to exit
	c.wg.Wait()

	close(c.quitChan)

	return
}

func (c *MuxConn) master() (net.Conn, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.connections == nil {
		return nil, net.ErrClosed
	}
	return c.connections[masterConnID].conn, nil
}

type chanConnPair struct {
	conn   net.Conn
	readCh chan Result
}

type connectionData struct {
	id   uint16
	data []byte
}

const lengthPrefixHeaderLen = 6

func decodeLen(b []byte) (int32, uint16) {
	return int32(b[3]) | int32(b[2])<<8 | int32(b[1])<<16 | int32(b[0])<<24,
		uint16(b[5]) | uint16(b[4])<<8
}

func encodeLen(b []byte, length int32, id uint16) {
	_ = b[3] // bounds check hint to compiler; see golang.org/issue/14808
	b[0] = byte(length >> 24 & 0x00FF)
	b[1] = byte(length >> 16 & 0x00FF)
	b[2] = byte(length >> 8 & 0x00FF)
	b[3] = byte(length)
	b[4] = byte(id >> 8 & 0x00FF)
	b[5] = byte(id)
}

func doWriteBuffer(writer io.Writer, msgBytes []byte, id uint16) (err error) {
	buf := make([]byte, len(msgBytes)+lengthPrefixHeaderLen)
	encodeLen(buf[:lengthPrefixHeaderLen], int32(len(msgBytes)), id)
	copy(buf[lengthPrefixHeaderLen:], msgBytes)
	_, err = writer.Write(buf)
	return
}

func doReadBuffer(reader io.Reader) (id uint16, buf []byte, err error) {
	var lengthPrefixBuf [lengthPrefixHeaderLen]byte

	_, err = io.ReadFull(reader, lengthPrefixBuf[:])
	if err != nil {
		return
	}

	var length int32
	length, id = decodeLen(lengthPrefixBuf[:])
	if length == 0 {
		return
	}
	buf = make([]byte, length)
	_, err = io.ReadFull(reader, buf)
	return
}

func (c *MuxConn) log(level logrus.Level, msg string, args ...interface{}) {
	logrus.
		WithFields(logrus.Fields{logging.KeyClass: "MuxConn"}).
		Logf(level, msg, args...)
}

func (c *MuxConn) read() {
	for {
		id, bytes, err := doReadBuffer(c.root)
		c.log(logrus.TraceLevel, "network connection read %d, %d, %v", id, len(bytes), err)
		if err == io.EOF {
			c.mu.Lock()
			defer c.mu.Unlock()
			for _, conn := range c.connections {
				close(conn.readCh) // force EOF on all connections
			}
			c.log(logrus.DebugLevel, "forced EOF on al muxed connections: %v", err)
			return
		}

		if err == nil && id == controlMessageID {
			c.readControlMessage(bytes)
			continue
		}

		c.mu.Lock()
		conn, ok := c.connections[id]
		c.mu.Unlock()
		if !ok {
			c.log(logrus.DebugLevel, "could not find connection with id %d", id)
			continue
		}

		// trust that the buffered chan has been configured correctly
		select {
		case conn.readCh <- Result{Error: err, Data: bytes}:
		case <-c.quitChan:
			return
		}
	}
}

func (c *MuxConn) writeChanConn(id uint16, write <-chan Result) {
	defer c.wg.Done()

	for {
		select {
		case <-c.quitChan:
			return
		case result, ok := <-write:
			if !ok {
				data, err := marshalControlMessage(controlMessage{ID: id, Type: controlTypeClose})
				if err != nil {
					c.log(logrus.ErrorLevel, "marshal control msg error: %v", err)
					return
				}
				err = doWriteBuffer(c.root, data, controlMessageID)
				if err != nil {
					c.log(logrus.DebugLevel, "write control msg net error: %v", err)
				}
				return
			}
			if result.Error != nil {
				c.log(logrus.DebugLevel, "channel result error: %v", result.Error)
				continue
			}
			err := doWriteBuffer(c.root, result.Data, id)
			if err != nil {
				c.log(logrus.DebugLevel, "write net error: %v", err)
			}
		}
	}
}

func unmarshalControlMessage(b []byte) (ret controlMessage, err error) {
	err = json.Unmarshal(b, &ret)
	return
}

func marshalControlMessage(msg controlMessage) ([]byte, error) {
	return json.Marshal(msg)
}

func (c *MuxConn) readControlMessage(bytes []byte) {
	msg, err := unmarshalControlMessage(bytes)
	if err != nil {
		c.log(logrus.DebugLevel, "could not scan control message len(%d): %v", len(bytes), err)
		return
	}

	switch msg.Type {
	case controlTypeClose:
		c.mu.Lock()
		defer c.mu.Unlock()
		conn, ok := c.connections[msg.ID]
		if !ok {
			c.log(logrus.DebugLevel, "could not find connection with id for closing read %d", msg.ID)
			return
		}
		close(conn.readCh)
		delete(c.connections, msg.ID)
	default:
		c.log(logrus.DebugLevel, "unknown message type %v", msg.Type)
	}
}

type controlMessage struct {
	ID   uint16
	Type controlType
}

type controlType uint8

const (
	controlTypeClose = iota
)
