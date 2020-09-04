package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
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
	httpAddr := flag.String("http", ":8000", "HTTP server listen address.")
	uiOnly := flag.Bool("ui", false, "Start in UI-only mode (disable I2C bus).")
	logFile := flag.String("log", "", "Log valid locations to file.")
	flag.Parse()

	u, err := url.Parse(*connStr)
	if err != nil {
		log.Fatal("ERROR: invalid server url: ", err)
	}
	topic := strings.TrimPrefix(u.Path, "/")
	u.Path = ""

	log.Println("Looking up host...")
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

	var enc *json.Encoder
	if *logFile != "" {
		fd, err := os.OpenFile(*logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND|os.O_SYNC, 0644)
		if err != nil {
			log.Fatal("failed to open log file: ", err)
		}
		defer fd.Close()

		enc = json.NewEncoder(fd)
		var start struct {
			Time  time.Time
			Event string
		}
		start.Time = time.Now()
		start.Event = "START"
		err = enc.Encode(start)
		if err != nil {
			log.Fatal("failed to log start event: ", err)
		}
	} else {
		enc = json.NewEncoder(ioutil.Discard)
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

	var pca *pca9685.Dev

	if !*uiOnly {
		_, err = host.Init()
		if err != nil {
			log.Fatal(err)
		}
		bus, err := i2creg.Open("")
		if err != nil {
			log.Fatal(err)
		}
		pca, err = pca9685.NewI2C(bus, pca9685.I2CAddr)
		if err != nil {
			log.Fatal(err)
		}
		pca.SetPwmFreq(50*physic.Hertz - (physic.Hertz / 2))
	}

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
		Lat, Lon       float64
		LatErr, LonErr float64
		Time           time.Time
	}
	var pLoc pontoonLocation
	const earthRadiusCM = 6371 * 1000 * 100
	orientAntenna := func() {
		if *uiOnly {
			return
		}
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
		log.Println("Distance:", dist)

		h := math.Sqrt(math.Pow(dist, 2) + math.Pow(cfg.Antenna.HeightCM, 2))
		yAdj := math.Asin(dist/h)*180/math.Pi - 90
		yAxis.SetAngle(yAdj)
		log.Println("Y-Angle:", yAdj)
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
		err = enc.Encode(newPLoc)
		if err != nil {
			log.Println("log location event:", err)
		}
	})
	if err != nil {
		log.Fatal(err)
	}

	srv := &server{cfg: &cfg, mx: &mx}
	http.HandleFunc("/", srv.serveIndex)
	http.HandleFunc("/save", func(w http.ResponseWriter, req *http.Request) {
		mx.Lock()
		c := cfg
		mx.Unlock()
		var err error
		setFloat := func(t *float64, name string) {
			if err != nil {
				return
			}
			var val float64
			val, err = strconv.ParseFloat(req.FormValue(name), 64)
			if err != nil {
				err = fmt.Errorf("invalid value for '%s': %v", name, err)
				return
			}
			*t = val
		}

		setFloat(&c.Antenna.Bearing, "bearing")
		setFloat(&c.Antenna.Lat, "lat")
		setFloat(&c.Antenna.Lng, "lng")
		setFloat(&c.Antenna.HeightCM, "height")

		setFloat(&c.ServoX.Offset, "xoffset")
		setFloat(&c.ServoX.Min, "xmin")
		setFloat(&c.ServoX.Max, "xmax")

		setFloat(&c.ServoY.Offset, "yoffset")
		setFloat(&c.ServoY.Min, "ymin")
		setFloat(&c.ServoY.Max, "ymax")

		c.Calibrate = req.FormValue("calibrate") == "1"
		c.CenterWhenOffline = req.FormValue("centeroffline") == "1"

		errRedir := func(err error) bool {
			if err == nil {
				return false
			}
			http.Redirect(w, req, "/?err="+url.QueryEscape(err.Error()), 302)
			return true
		}
		if errRedir(err) {
			return
		}

		data, err := json.MarshalIndent(c, "", "  ")
		if errRedir(err) {
			return
		}

		mx.Lock()
		err = cli.Publish(path.Join(topic, "config"), 2, true, data)
		if errRedir(err) {
			return
		}
		cfg = c
		mx.Unlock()

		orientAntenna()
		http.Redirect(w, req, "/", 302)
	})

	err = http.ListenAndServe(*httpAddr, nil)
	if err != nil {
		log.Fatal("ERROR: ", err)
	}
}
