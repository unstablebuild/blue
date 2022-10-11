package gps

import (
	"context"
	"time"

	"github.com/ernestrc/blue/document"
)

const deviceID = "segador-1"

var (
	fixtureCoords = Coordinates{
		Latitude:  3.4,
		Longitude: 4.5,
		Altitude:  11.0,
		DeviceID:  deviceID,
		UnixTime:  1596141954,
	}

	c1     = fixtureCoords
	c2     = fixtureCoords
	c3     = fixtureCoords
	second = time.Unix(c1.UnixTime, 0).Add(-time.Minute)
	first  = time.Unix(c1.UnixTime, 0).Add(-5 * time.Minute)
	c4     = Coordinates{}
)

func init() {
	c1.UnixTime = first.Unix()

	c2.UnixTime = second.Unix()

	c3.DeviceID = "2"
	c3.UnixTime = second.Unix()

	dayBefore := time.Unix(fixtureCoords.UnixTime, 0).Add(-24 * time.Hour).Unix()
	c4 = Coordinates{DeviceID: deviceID, UnixTime: dayBefore}
}

type failingDocService struct {
	err error
}

func (s *failingDocService) Create(
	ctx context.Context, ID string, doc interface{},
) error {
	return s.err
}
func (s *failingDocService) Set(
	ctx context.Context, ID string, doc interface{},
) error {
	return s.err
}
func (s *failingDocService) Update(
	ctx context.Context, ID string, updates []document.Update,
) error {
	return s.err
}
func (s *failingDocService) Get(
	ctx context.Context, ID string, doc interface{},
) error {
	return s.err
}
func (s *failingDocService) Delete(
	ctx context.Context, ID string,
) error {
	return s.err
}
func (s *failingDocService) List(
	ctx context.Context, filters []document.Filter,
) (document.Iterator, error) {
	return nil, s.err
}
func (s *failingDocService) Close() error {
	return s.err
}
