package gps

import "context"

type staticPositioner struct {
	pos Coordinates
}

// NewStaticPositioner returns a Positioner that always yields the same
// GPS position.
func NewStaticPositioner(pos Coordinates) Positioner {
	return staticPositioner{pos: pos}
}

func (p staticPositioner) Position(ctx context.Context) (Coordinates, error) {
	return p.pos, nil
}

func (p staticPositioner) Close() error {
	return nil
}
