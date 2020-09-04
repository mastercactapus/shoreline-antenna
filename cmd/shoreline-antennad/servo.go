package main

import (
	"math"

	"github.com/golang/geo/s2"
	"periph.io/x/periph/conn/gpio"
	"periph.io/x/periph/experimental/devices/pca9685"
)

const degreesPerSec = 60.0

type servo struct {
	pca *pca9685.Dev

	channel  int
	sweep    float64
	inverted bool

	cfg *ServoConfig

	lastAngle float64
}

func clamp(v, min, max float64) float64 {
	return math.Min(math.Max(v, min), max)
}

func (s *servo) SetOffset(angle float64) {
	s.cfg.Offset = angle

	if s.lastAngle == 0 {
		return
	}

	s.SetAngle(s.lastAngle)
}
func (s *servo) SetAngle(angle float64) {
	angle += s.cfg.Offset

	// allow setting boundaries
	if s.cfg.Max != 0 {
		angle = math.Min(angle, s.cfg.Max)
	}
	if s.cfg.Min != 0 {
		angle = math.Max(angle, s.cfg.Min)
	}

	angle = clamp(angle, -s.sweep/2, s.sweep/2) // clamp to +/- half the sweep angle

	if s.inverted {
		angle = -angle
	}

	duty := math.Min(0.025+(angle+(s.sweep/2))/s.sweep*.1, 0.125) // 2.5% and 12.5% duty cycle @ 50hz = 500us-2500us

	s.pca.SetPwm(s.channel, 0, gpio.Duty(duty*4096))
	// var diff float64
	// if s.lastAngle == 0 {
	// 	diff = (s.max - s.min) / 2
	// } else {
	// 	diff = math.Abs(s.lastAngle - angle)
	// }
	s.lastAngle = angle
	// time.Sleep(time.Millisecond * time.Duration(diff*1000/degreesPerSec))
}
func (s *servo) Center() {
	s.SetAngle(0)
}

func calcAngle(heading, lat1, lon1, lat2, lon2 float64) float64 {
	dLon := lon2 - lon1

	y := math.Sin(dLon) * math.Cos(lat2)
	x := math.Cos(lat1)*math.Sin(lat2) - math.Sin(lat1)*math.Cos(lat2)*math.Cos(dLon)

	b := math.Atan2(y, x)

	a := b * 180 / math.Pi
	// a = math.Mod(a+360, 360)
	// a = 360 - a

	return a
}

var northPole = s2.PointFromLatLng(s2.LatLngFromDegrees(90, 0))
var southPole = s2.PointFromLatLng(s2.LatLngFromDegrees(-90, 0))
