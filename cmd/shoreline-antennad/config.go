package main

type Config struct {
	Antenna struct {
		Bearing  float64
		HeightCM float64
		Lat, Lng float64
	}

	ServoX, ServoY ServoConfig

	CenterWhenOffline bool

	Calibrate bool
}

type ServoConfig struct {
	Offset   float64
	Min, Max float64
}
