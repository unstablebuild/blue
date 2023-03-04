package net

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	readChanBuffer          = 1
	writeChanBuffer         = 1
	controlMessageID        = 0
	masterConnID            = 1
	acceptBackpressureThres = 10
)

var (
	_ net.Conn     = (*MuxConn)(nil)
	_ net.Listener = (*MuxConn)(nil)
)

// MuxConn implements a net.Conn capable of multiplexing
// multiple logical connections, bidirectionally, over the
// same underlying net.Conn. It also satisfies net.Listener.
type MuxConn struct {
	root     net.Conn
	nextID   uint16
	quitCh   chan struct{}
	acceptCh chan net.Conn
	dialChs  map[uint16]chan bool

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
	ret.quitCh = make(chan struct{})
	ret.acceptCh = make(chan net.Conn, acceptBackpressureThres)
	ret.dialChs = make(map[uint16]chan bool)
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

// Dial dials to a MuxConn with the next random available ID
// using a context.Context that never cancels.
func (c *MuxConn) Dial() (net.Conn, error) {
	return c.DialContext(context.Background())
}

// DialContext dials to a MuxConn with the next random available ID.
func (c *MuxConn) DialContext(ctx context.Context) (net.Conn, error) {
	c.mu.Lock()
	id := c.nextID
	c.nextID++
	c.mu.Unlock()
	return c.DialConnContext(ctx, id)
}

// DialConn dials to the other end of this MuxConn with a context
// that never cancels. See DialConnContext for more details.
func (c *MuxConn) DialConn(id uint16) (net.Conn, error) {
	return c.DialConnContext(context.Background(), id)
}

// DialConnContext dials to the other end of this MuxConn for a connection
// with the given id. If the connection does not exist, an error
// is returned on the next call to Read or Write. The context is used
// to timeout the establishment of the connection.
func (c *MuxConn) DialConnContext(ctx context.Context, id uint16) (net.Conn, error) {
	c.mu.Lock()

	_, ok := c.connections[id]
	if ok {
		// NOTE: we could, but makes some of the read chan logic hard to
		// reason about.
		c.mu.Unlock()
		return nil, errors.New("cannot dial twice to the same connection")
	}

	ch := make(chan bool)
	c.dialChs[id] = ch
	c.mu.Unlock()

	data, err := marshalControlMessage(controlMessage{ID: id, Type: controlTypeInit})
	if err != nil {
		return nil, fmt.Errorf("could not marshal init message %v", err)
	}
	err = doWriteWriter(c.root, data, controlMessageID)
	if err != nil {
		return nil, fmt.Errorf("could not write init control msg: %v", err)
	}

	select {
	case ok := <-ch:
		if !ok {
			return nil, errors.New("connection was not accepted")
		}
		c.mu.Lock()
		defer c.mu.Unlock()
		return c.newChanConn(id), nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (c *MuxConn) newChanConn(id uint16) net.Conn {
	read := make(chan ReadResult, readChanBuffer)
	write := make(chan ReadResult, writeChanBuffer)
	chanConn := ChanConn(c.root.LocalAddr(), c.root.RemoteAddr(), read, write)
	pai := chanConnPair{
		readCh: read,
		conn:   chanConn,
	}
	c.connections[id] = pai
	c.wg.Add(1)
	go c.writeChanConn(id, write)
	return chanConn
}

// Mux creates a new net.Conn on this end of the connection
// which can be dialed on the other end of the connection via Dial.
func (c *MuxConn) Mux() (uint16, net.Conn, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	id := c.nextID
	chanConn := c.newChanConn(id)
	c.nextID++
	return id, chanConn, nil
}

// Accept waits for and returnds the next connection to the listener.
func (c *MuxConn) Accept() (net.Conn, error) {
	conn := <-c.acceptCh
	return conn, nil
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

// Addr returns the virtual listener's address. It is equivalent
// to LocalAddr.
func (c *MuxConn) Addr() net.Addr {
	return c.LocalAddr()
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

	close(c.quitCh)

	return
}

func (c *MuxConn) master() (net.Conn, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.connections == nil {
		return nil, net.ErrClosed
	}
	masterConn, ok := c.connections[masterConnID]
	if !ok {
		return nil, net.ErrClosed
	}
	return masterConn.conn, nil
}

type chanConnPair struct {
	conn   net.Conn
	readCh chan ReadResult
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
	_ = b[5] // bounds check hint to compiler; see golang.org/issue/14808
	b[0] = byte(length >> 24 & 0x00FF)
	b[1] = byte(length >> 16 & 0x00FF)
	b[2] = byte(length >> 8 & 0x00FF)
	b[3] = byte(length)
	b[4] = byte(id >> 8 & 0x00FF)
	b[5] = byte(id)
}

func doWriteWriter(writer io.Writer, msgBytes []byte, id uint16) (err error) {
	buf := make([]byte, len(msgBytes)+lengthPrefixHeaderLen)
	encodeLen(buf[:lengthPrefixHeaderLen], int32(len(msgBytes)), id)
	copy(buf[lengthPrefixHeaderLen:], msgBytes)
	_, err = writer.Write(buf)
	return
}

func doReadReader(reader io.Reader) (id uint16, idOk bool, buf []byte, err error) {
	var lengthPrefixBuf [lengthPrefixHeaderLen]byte

	_, err = io.ReadFull(reader, lengthPrefixBuf[:])
	if err != nil {
		return
	}

	var length int32
	length, id = decodeLen(lengthPrefixBuf[:])
	idOk = true
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
		id, idOk, bytes, err := doReadReader(c.root)
		if err == io.EOF {
			c.mu.Lock()
			for _, conn := range c.connections {
				close(conn.readCh) // force EOF on all connections
			}
			c.mu.Unlock()
			c.log(logrus.DebugLevel, "forced EOF on al muxed connections: %v", err)
			return
		}

		if err != nil && !idOk {
			c.log(logrus.DebugLevel, "unexpected error while reading header %v", err)
			return
		}

		if err == nil && id == controlMessageID {
			err := c.readControlMessage(bytes)
			if err != nil {
				c.log(logrus.ErrorLevel, "failed reading control message: %v", err)
			}
			continue
		}

		c.mu.Lock()
		conn, ok := c.connections[id]
		c.mu.Unlock()
		if !ok {
			c.log(logrus.DebugLevel, "could not find connection with id %d", id)
			continue
		}

		res := ReadResult{Error: err, Data: bytes, Ch: make(chan struct{})}
		select {
		case conn.readCh <- res:
			select {
			case <-c.quitCh:
				return
			case <-res.Ch:
			}
		case <-c.quitCh:
			return
		}
	}
}

func (c *MuxConn) writeChanConn(id uint16, write <-chan ReadResult) {
	defer c.wg.Done()

	for {
		select {
		case <-c.quitCh:
			return
		case result, ok := <-write:
			if !ok {
				data, err := marshalControlMessage(controlMessage{ID: id, Type: controlTypeClose})
				if err != nil {
					c.log(logrus.ErrorLevel, "marshal control msg error: %v", err)
					return
				}
				err = doWriteWriter(c.root, data, controlMessageID)
				if err != nil {
					c.log(logrus.DebugLevel, "write control msg net error: %v", err)
				}
				return
			}
			if result.Error != nil {
				c.log(logrus.DebugLevel, "channel result error: %v", result.Error)
				close(result.Ch)
				continue
			}
			err := doWriteWriter(c.root, result.Data, id)
			// unblock write such that Close is now safe to call
			// in that it would follow the same semantics as net.Conn
			close(result.Ch)
			if err != nil {
				c.log(logrus.DebugLevel, "write net error: %v", err)
			}
		}
	}
}

func unmarshalControlMessage(b []byte) (ret controlMessage, err error) {
	// NOTE: should probably use proto or a more efficient protocol.
	err = json.Unmarshal(b, &ret)
	return
}

func marshalControlMessage(msg controlMessage) ([]byte, error) {
	return json.Marshal(msg)
}

func (c *MuxConn) readControlMessage(bytes []byte) error {
	msg, err := unmarshalControlMessage(bytes)
	if err != nil {
		err = fmt.Errorf("could not scan control message len(%d): %v", len(bytes), err)
		return err
	}

	switch msg.Type {
	case controlTypeClose:
		c.mu.Lock()
		conn, ok := c.connections[msg.ID]
		if !ok {
			c.mu.Unlock()
			c.log(logrus.WarnLevel, "could not find connection with id for closing read %d", msg.ID)
			return nil
		}
		close(conn.readCh)
		delete(c.connections, msg.ID)
		c.mu.Unlock()
		return nil
	case controlTypeInit:
		c.mu.Lock()
		conn, ok := c.connections[msg.ID]
		if !ok {
			c.newChanConn(msg.ID)
			conn, _ = c.connections[msg.ID]
		}
		c.mu.Unlock()

		select {
		case c.acceptCh <- conn.conn:
			data, err := marshalControlMessage(controlMessage{ID: msg.ID, Type: controlTypeInitAck})
			if err != nil {
				err = fmt.Errorf("failed to marshal control message: %v", err)
				return err
			}
			err = doWriteWriter(c.root, data, controlMessageID)
			if err != nil {
				err = fmt.Errorf("failed accepting init control message: %v", err)
				return err
			}
			return nil
		default:
			msg := controlMessage{ID: msg.ID, Type: controlTypeInitAck, Err: true}
			data, werr := marshalControlMessage(msg)
			if werr != nil {
				err = multierr.Append(err, werr)
				return err
			}
			err = doWriteWriter(c.root, data, controlMessageID)
			if err != nil {
				err = fmt.Errorf("failed rejecting init control message: %v", err)
				return err
			}
			return nil
		}
	case controlTypeInitAck:
		c.mu.Lock()
		dialCh, ok := c.dialChs[msg.ID]
		delete(c.dialChs, msg.ID)
		c.mu.Unlock()
		if !ok {
			c.log(logrus.WarnLevel, "could not find any dial ch waiting for ack: %v", msg.ID)
			return nil
		}
		select {
		case dialCh <- !msg.Err:
		default:
		}
		return nil
	default:
		err = fmt.Errorf("unknown message type %v", msg.Type)
		return err
	}
}

type controlMessage struct {
	ID   uint16
	Type controlType
	Err  bool
}

type controlType uint8

const (
	controlTypeClose = iota
	controlTypeInit
	controlTypeInitAck
)
