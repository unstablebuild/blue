package gps

import (
	"bufio"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const rawMsg = `$GPTXT,01,01,02,u-blox ag - www.u-blox.com*50
$GPTXT,01,01,02,HW  UBX-G70xx   00070000 *77
$GPTXT,01,01,02,ROM CORE 1.00 (59842) Jun 27 2012 17:43:52*59
$GPTXT,01,01,02,PROTVER 14.00*1E
$GPTXT,01,01,02,ANTSUPERV=AC SD PDoS SR*20
$GPTXT,01,01,02,ANTSTATUS=OK*3B
$GPTXT,01,01,02,LLC FFFFFFFF-FFFFFFFD-FFFFFFFF-FFFFFFFF-FFFFFFF9*53
$GPRMC,224737.00,A,3739.70270,N,12152.36695,W,0.250,,010820,,,D*6A
$GPVTG,,T,,M,0.250,N,0.463,K,D*20
$GPGGA,224737.00,3739.70270,N,12152.36695,W,2,09,1.03,116.1,M,-29.4,M,,0000*62
$GPGSA,A,3,46,10,32,20,27,21,51,15,18,,,,2.23,1.03,1.98*0C
$GPGSV,3,1,12,08,13,321,18,10,65,276,29,13,00,031,,15,24,047,28*7A
$GPGSV,3,2,12,18,51,085,20,20,69,021,25,21,53,317,21,27,43,305,25*70
$GPGSV,3,3,12,29,06,154,,32,22,190,34,46,46,192,38,51,44,157,32*7E
$GPGLL,3739.70270,N,12152.36695,W,224737.00,A,D*7F
$GPRMC,224738.00,A,3739.70273,N,12152.36701,W,0.098,,010820,,,D*6C
$GPVTG,,T,,M,0.098,N,0.182,K,D*2C
$GPGGA,224738.00,3739.70273,N,12152.36701,W,2,09,1.03,116.0,M,-29.4,M,,0000*63
$GPGSA,A,3,46,10,32,20,27,21,51,15,18,,,,2.23,1.03,1.98*0C
$GPGSV,3,1,12,08,13,321,18,10,65,276,29,13,00,031,,15,24,047,28*7A
$GPGSV,3,2,12,18,51,085,21,20,69,021,25,21,53,317,21,27,43,305,25*71
$GPGSV,3,3,12,29,06,154,,32,22,190,34,46,46,192,37,51,44,157,32*71
$GPGLL,3739.70273,N,12152.36701,W,224738.00,A,D*7F
$GPRMC,224739.00,A,3739.70278,N,12152.36709,W,0.091,,010820,,,D*67
$GPVTG,,T,,M,0.091,N,0.169,K,D*20
$GPGGA,224739.00,3739.70278,N,12152.36709,W,2,09,1.03,115.9,M,-29.4,M,,0000*6B
$GPGSA,A,3,46,10,32,20,27,21,51,15,18,,,,2.23,1.03,1.98*0C
$GPGSV,3,1,12,08,13,321,18,10,65,276,28,13,00,031,,15,24,047,28*7B
$GPGSV,3,2,12,18,51,085,20,20,69,021,25,21,53,317,22,27,43,305,25*73
$GPGSV,3,3,12,29,06,154,,32,22,190,34,46,46,192,37,51,44,157,32*71
$GPGLL,3739.70278,N,12152.36709,W,224739.00,A,D*7D
$GPRMC,224740.00,A,3739.70279,N,12152.36714,W,0.068,,010820,,,D*62
`

const testID = "watchful-on-call"

var (
	expected1 = Coordinates{
		DeviceID:  testID,
		Latitude:  37.661713,
		Longitude: -121.87278,
		Altitude:  116.1,
	}
	expected2 = Coordinates{
		DeviceID:  testID,
		Latitude:  37.661713,
		Longitude: -121.87278,
		Altitude:  116,
	}
	expected3 = Coordinates{
		DeviceID:  testID,
		Latitude:  37.661713,
		Longitude: -121.87279,
		Altitude:  115.9,
	}
)

func assertEqualCoordinates(t *testing.T, expected, actual Coordinates) {
	assert.True(t, time.Unix(actual.UnixTime, 0).After(time.Now().Add(-1*time.Minute)))
	actual.UnixTime = 0
	expected.UnixTime = 0
	require.Equal(t, expected, actual)
}

func newTestSerialDevicePositioner(t *testing.T) Positioner {
	return &serialDevicePositioner{
		id:      testID,
		scanner: bufio.NewScanner(strings.NewReader(rawMsg)),
	}
}

func TestSerialPositioner(t *testing.T) {
	p := newTestSerialDevicePositioner(t)
	ctx := context.Background()

	pos, err := p.Position(ctx)
	require.NoError(t, err)
	assertEqualCoordinates(t, expected1, pos)

	pos, err = p.Position(ctx)
	require.NoError(t, err)
	assertEqualCoordinates(t, expected2, pos)

	pos, err = p.Position(ctx)
	require.NoError(t, err)
	assertEqualCoordinates(t, expected3, pos)
}
