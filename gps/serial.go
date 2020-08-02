package gps

import (
	"bufio"
	"context"
	"io"
	"time"

	"github.com/adrianmo/go-nmea"
	"github.com/ernestrc/blue/logging"
	"github.com/jacobsa/go-serial/serial"
	log "github.com/sirupsen/logrus"
)

const (
	constructorTimeout = 5 * time.Second
	gptxSeq            = "$GPTXT"
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
		id:         id,
		scanner:    scanner,
		serialPort: serialPort,
	}

	// test positioner
	ctx, cancel := context.WithTimeout(context.Background(), constructorTimeout)
	defer cancel()

	_, err = p.Position(ctx)
	if err != nil {
		return nil, err
	}

	return p, nil
}

func (p *serialDevicePositioner) Position(ctx context.Context) (Coordinates, error) {
	for {
		ok := p.scanner.Scan()
		if !ok {
			return Coordinates{}, io.EOF
		}

		msg := p.scanner.Text()
		if len(msg) < len(gptxSeq) || msg[:len(gptxSeq)] == gptxSeq {
			continue
		}

		logFields := log.Fields{"Raw": msg, logging.KeyCallType: "ParseNMEA"}

		s, err := nmea.Parse(msg)
		if err != nil {
			logFields[logging.KeyStep] = logging.ValueStepFailure
			logFields[logging.KeyError] = err.Error()
			log.WithFields(logFields).Error()
			return Coordinates{}, err
		}

		if s.DataType() != nmea.TypeGGA {
			continue
		}

		data := s.(nmea.GGA)
		logFields[logging.KeyStep] = logging.ValueStepSuccess
		logFields["Data"] = data.String()
		log.WithFields(logFields).Trace()

		return Coordinates{
			DeviceID:  p.id,
			Latitude:  float32(data.Latitude),
			Longitude: float32(data.Longitude),
			Altitude:  float32(data.Altitude),
			UnixTime:  time.Now().Unix(),
		}, nil
	}
}

func (p *serialDevicePositioner) Close() error {
	return p.serialPort.Close()
}
