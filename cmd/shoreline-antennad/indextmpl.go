package main

import (
	"html/template"
)

var tmpl = template.Must(template.New("index").Parse(`<html>
  <head>
    <title>Shoreline Antenna</title>
    <link
      rel="stylesheet"
      href="https://cdnjs.cloudflare.com/ajax/libs/semantic-ui/2.4.1/semantic.min.css"
      integrity="sha512-8bHTC73gkZ7rZ7vpqUQThUDhqcNFyYi2xgDgPDHc+GXVGHXq+xPjynxIopALmOPqzo9JZj0k6OqqewdGO3EsrQ=="
      crossorigin="anonymous"
    />
    <script
      src="https://code.jquery.com/jquery-3.1.1.min.js"
      integrity="sha256-hVVnYaiADRTO2PzUGmuLJr8BLUSjGIZsDYGmIJLv2b8="
      crossorigin="anonymous"
    ></script>
    <script
      src="https://cdnjs.cloudflare.com/ajax/libs/semantic-ui/2.4.1/semantic.min.js"
      integrity="sha512-dqw6X88iGgZlTsONxZK9ePmJEFrmHwpuMrsUChjAw1mRUhUITE5QU9pkcSox+ynfLhL15Sv2al5A0LVyDCmtUw=="
      crossorigin="anonymous"
    ></script>
  </head>

  <body>
    <div class="ui text container">
      <form class="ui form" action="/save" method="POST">
        <h4 class="ui dividing header">Antenna</h4>
        <div class="field">
          <div class="fields">
            <div class="field">
              <label>Bearing</label>
              <input
                type="number"
                name="bearing"
                value="{{.Antenna.Bearing}}"
              />
            </div>
            <div class="field">
              <label>Height</label>
              <input
                type="number"
                name="height"
                value="{{.Antenna.HeightCM}}"
              />
            </div>
            <div class="field">
              <label>Latitiude</label>
              <input type="number" name="lat" value="{{.Antenna.Lat}}" />
            </div>
            <div class="field">
              <label>Longitude</label>
              <input type="number" name="lng" value="{{.Antenna.Lng}}" />
            </div>
          </div>
        </div>

        <h4 class="ui dividing header">Horizontal Servo (X)</h4>
        <div class="field">
          <div class="fields">
            <div class="field">
              <label>Offset</label>
              <input type="number" name="xoffset" value="{{.ServoX.Offset}}" />
            </div>
            <div class="field">
              <label>Min</label>
              <input type="number" name="xmin" value="{{.ServoX.Min}}" />
            </div>
            <div class="field">
              <label>Max</label>
              <input type="number" name="xmax" value="{{.ServoX.Max}}" />
            </div>
          </div>
        </div>

        <h4 class="ui dividing header">Vertical Servo (Y)</h4>
        <div class="field">
          <div class="fields">
            <div class="field">
              <label>Offset</label>
              <input type="number" name="yoffset" value="{{.ServoY.Offset}}" />
            </div>
            <div class="field">
              <label>Min</label>
              <input type="number" name="ymin" value="{{.ServoY.Min}}" />
            </div>
            <div class="field">
              <label>Max</label>
              <input type="number" name="ymax" value="{{.ServoY.Max}}" />
            </div>
          </div>
        </div>

        <h4 class="ui dividing header">Other</h4>
        <div class="inline field">
          <div class="ui toggle checkbox">
            <input
              type="checkbox"
              tabindex="0"
              value="1"
              name="centeroffline"
              {{if .CenterWhenOffline}}checked{{end}}
              class="hidden"
            />
            <label>Center When Offline</label>
          </div>
        </div>
        <div class="inline field">
          <div class="ui toggle checkbox">
            <input
              type="checkbox"
              tabindex="0"
              value="1"
              name="calibrate"
              {{if .Calibrate}}checked{{end}}
              class="hidden"
            />
            <label>Calibration Mode</label>
          </div>
        </div>

        <button class="ui button" type="submit">Save</button>
      </form>
    </div>
    <script>
      $(".ui.checkbox").checkbox();
    </script>
  </body>
</html>`))
