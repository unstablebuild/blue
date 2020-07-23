package gps

import "context"

type staticPositioner struct {
	pos Coordinates
}

// NewStaticPosition returns a Positioner that always yields the same
// GPS position.
func NewStaticPosition(pos Coordinates) Positioner {
	return staticPositioner{pos: pos}
}

func (p staticPositioner) Position(ctx context.Context) (Coordinates, error) {
	return p.pos, nil
}

func (p staticPositioner) Close() error {
	return nil
}
