package gps

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/adrianmo/go-nmea"
	"github.com/jacobsa/go-serial/serial"
)

type serialDevicePositioner struct {
	id         string
	serialPort io.ReadWriteCloser
	scanner    *bufio.Scanner
}

// NewSerialDevicePositioner returns a GPS positioner that reads a GPS position
// out of NMEA GGA sequences out of a serial device, specified via opts. It uses
// id as Coordinates DeviceID.
func NewSerialDevicePositioner(id string, opts serial.OpenOptions) (
	Positioner, error,
) {
	serialPort, err := serial.Open(opts)
	if err != nil {
		return nil, err
	}
	reader := bufio.NewReader(serialPort)
	scanner := bufio.NewScanner(reader)

	p := &serialDevicePositioner{
		scanner:    scanner,
		serialPort: serialPort,
	}

	// test positioner
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = p.Position(ctx)
	if err != nil {
		return nil, err
	}

	p.id = id

	return p, nil
}

func (p *serialDevicePositioner) Position(ctx context.Context) (Coordinates, error) {
	ok := p.scanner.Scan()
	if !ok {
		return Coordinates{}, io.EOF
	}

	s, err := nmea.Parse(p.scanner.Text())
	if err != nil {
		return Coordinates{}, err
	}

	if s.DataType() != nmea.TypeGGA {
		return Coordinates{}, fmt.Errorf("invalid NMEA type: %s", s.DataType())
	}

	data := s.(nmea.GGA)
	return Coordinates{
		DeviceID:  p.id,
		Latitude:  float32(data.Latitude),
		Longitude: float32(data.Longitude),
		Altitude:  float32(data.Altitude),
		Time:      time.Now(),
	}, nil
}

func (p *serialDevicePositioner) Close() error {
	return p.serialPort.Close()
}
