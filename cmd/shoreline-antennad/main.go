package main

import (
	"encoding/json"
	"flag"
	"log"
	"math"
	"net/url"
	"os"
	"os/signal"
	"path"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/golang/geo/s2"
	uuid "github.com/satori/go.uuid"
	"periph.io/x/periph/conn/i2c/i2creg"
	"periph.io/x/periph/conn/physic"
	"periph.io/x/periph/experimental/devices/pca9685"
	"periph.io/x/periph/host"
)

func main() {
	connStr := flag.String("s", "tcp://homeassistant.local:1883/shorelinepi", "Connection url, leave host blank to use service discovery.")
	pontoonTopicRoot := flag.String("p", "pontoonpi", "Root name of the pontoon topic.")
	id := flag.String("c", "shoreline-antennad-"+uuid.NewV4().String(), "Client ID.")
	flag.Parse()

	u, err := url.Parse(*connStr)
	if err != nil {
		log.Fatal("ERROR: invalid server url: ", err)
	}
	topic := strings.TrimPrefix(u.Path, "/")
	u.Path = ""

	var hosts []string
	if u.Host != "" {
		hosts, err = resolveLookup(u.Host)
		if err != nil {
			log.Fatal("ERROR: resolve lookup: ", err)
		}
	} else {
		hosts, err = mdnsLookup("_mqtt._tcp")
		if err != nil {
			log.Fatal("ERROR: mdns lookup: ", err)
		}
	}

	opts := mqtt.NewClientOptions()
	opts.SetClientID(*id)
	opts.SetWill(path.Join(topic, "online"), "false", 1, true)
	for _, u.Host = range hosts {
		log.Println("Adding broker:", u.String())
		opts.AddBroker(u.String())
	}
	cli := goClient{Client: mqtt.NewClient(opts)}
	if err := cli.Connect(); err != nil {
		log.Fatal("ERROR: connect: ", err)
	}
	err = cli.Publish(path.Join(topic, "online"), 0, true, []byte("true"))
	if err != nil {
		log.Fatal("ERROR: publish: ", err)
	}

	_, err = host.Init()
	if err != nil {
		log.Fatal(err)
	}
	bus, err := i2creg.Open("")
	if err != nil {
		log.Fatal(err)
	}
	pca, err := pca9685.NewI2C(bus, pca9685.I2CAddr)
	if err != nil {
		log.Fatal(err)
	}
	pca.SetPwmFreq(50*physic.Hertz - (physic.Hertz / 2))

	var cfg Config
	yAxis := &servo{
		pca:   pca,
		sweep: 90,
		cfg:   &cfg.ServoY,
		// min:      -20,
		// max:      20,
		channel: 1,
		// offset:   0,
		inverted: true,
	}

	xAxis := &servo{
		pca:   pca,
		sweep: 135,
		// min:     -45,
		// max:     45,
		channel: 0,
		// offset:  -10,
		cfg: &cfg.ServoX,
	}

	var mx sync.Mutex
	var pOnline bool
	type pontoonLocation struct {
		Lat, Lon float64
		Time     time.Time
	}
	var pLoc pontoonLocation
	const earthRadiusCM = 6371 * 1000 * 100
	orientAntenna := func() {
		if cfg.Calibrate {
			log.Println("Calibration mode.")
			xAxis.Center()
			yAxis.Center()
			return
		}
		if cfg.Antenna.Lat == 0 || cfg.Antenna.Lng == 0 {
			log.Println("ERROR: ignoring point as antenna is not configured.")
			return
		}

		if !pOnline {
			if cfg.CenterWhenOffline {
				xAxis.Center()
				yAxis.Center()
			}

			return
		}

		if pLoc.Time.IsZero() {
			log.Println("ERROR: ignoring point as location isn't received yet.")
			return
		}

		dest := s2.PointFromLatLng(s2.LatLngFromDegrees(pLoc.Lat, pLoc.Lon))
		origin := s2.PointFromLatLng(s2.LatLngFromDegrees(cfg.Antenna.Lat, cfg.Antenna.Lng))

		tAng := -s2.TurnAngle(southPole, origin, dest).Degrees()

		b := cfg.Antenna.Bearing
		if b >= 180 {
			b -= 360
		}

		newAngle := tAng - b
		xAxis.SetAngle(newAngle)
		log.Println("X-Angle", newAngle)

		dist := origin.Distance(dest).Abs().Radians() * earthRadiusCM
		log.Println("dist:", dist)

		h := math.Sqrt(math.Pow(dist, 2) + math.Pow(cfg.Antenna.HeightCM, 2))
		yAdj := math.Asin(dist/h)*180/math.Pi - 90
		yAxis.SetAngle(yAdj)
		log.Println("angle:", yAdj)
	}

	err = cli.Subscribe(path.Join(topic, "config"), 1, func(_ mqtt.Client, msg mqtt.Message) {
		mx.Lock()
		defer mx.Unlock()
		msg.Ack()

		var newCfg Config
		err := json.Unmarshal(msg.Payload(), &newCfg)
		if err != nil {
			log.Println("ERROR: invalid config:", err)
			return
		}
		cfg = newCfg

		orientAntenna()
		log.Println("Config updated.")
	})
	if err != nil {
		log.Fatal(err)
	}

	err = cli.Subscribe(path.Join(*pontoonTopicRoot, "online"), 0, func(_ mqtt.Client, msg mqtt.Message) {
		mx.Lock()
		defer mx.Unlock()
		msg.Ack()

		pOnline = string(msg.Payload()) == "true"

		orientAntenna()
		log.Println("Pontoon Online =", pOnline)
	})
	if err != nil {
		log.Fatal(err)
	}
	err = cli.Subscribe(path.Join(*pontoonTopicRoot, "location"), 1, func(_ mqtt.Client, msg mqtt.Message) {
		mx.Lock()
		defer mx.Unlock()
		msg.Ack()

		var newPLoc pontoonLocation
		err := json.Unmarshal(msg.Payload(), &newPLoc)
		if err != nil {
			log.Println("ERROR: invalid config:", err)
			return
		}
		pLoc = newPLoc

		orientAntenna()
		log.Println("Location updated.")
	})
	if err != nil {
		log.Fatal(err)
	}

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	<-ch

}
