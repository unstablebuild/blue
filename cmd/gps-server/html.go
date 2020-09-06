package main

const template = `
<html>
  <head>
	<script src="https://cdnjs.cloudflare.com/ajax/libs/leaflet/1.6.0/leaflet.js" integrity="sha512-gZwIG9x3wUXg2hdXF6+rVkLF/0Vi9U8D2Ntg4Ga5I5BZpVkVxlJWbSQtXPSiUTtC0TjtGOmxa1AJPuV0CPthew==" crossorigin="anonymous"></script>
	<script src="https://cdnjs.cloudflare.com/ajax/libs/leaflet-routing-machine/3.2.12/leaflet-routing-machine.min.js" integrity="sha512-FW2A4pYfHjQKc2ATccIPeCaQpgSQE1pMrEsZqfHNohWKqooGsMYCo3WOJ9ZtZRzikxtMAJft+Kz0Lybli0cbxQ==" crossorigin="anonymous"></script>
	<link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/leaflet-routing-machine/3.2.12/leaflet-routing-machine.css" integrity="sha512-eD3SR/R7bcJ9YJeaUe7KX8u8naADgalpY/oNJ6AHvp1ODHF3iR8V9W4UgU611SD/jI0GsFbijyDBAzSOg+n+iQ==" crossorigin="anonymous" />
	<link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/leaflet/1.6.0/leaflet.css" integrity="sha512-xwE/Az9zrjBIphAcBb3F6JVqxf46+CDLwfLMHloNu6KEQCAWi6HcDUbeOfBIptF7tcCzusKFjFw2yuvEpDL9wQ==" crossorigin="anonymous" />
	<script src="https://cdnjs.cloudflare.com/ajax/libs/jquery/3.5.1/jquery.min.js" integrity="sha512-bLT0Qm9VnAYZDflyKcBaQ2gg0hSYNQrJ8RilYldYQ1FxQYoCLtUjuuRuZo+fjqhx/qtq/1itJ0C2ejDxltZVFg==" crossorigin="anonymous"></script>
	<script src="https://cdnjs.cloudflare.com/ajax/libs/moment.js/2.27.0/moment.min.js" integrity="sha512-rmZcZsyhe0/MAjquhTgiUcb4d9knaFc7b5xAfju483gbEXTkeJRUMIPk6s3ySZMYUHEcjKbjLjyddGWMrNEvZg==" crossorigin="anonymous"></script>
    <script type="text/javascript">

	  function makeLabel(o) {
		  var t = moment.unix(o.UnixTime)
		  var lastSeen = t.fromNow();
		  var timePretty = t.format("YYYY-MM-DD HH:mm:ss Z");
		  return o.DeviceID + " " + lastSeen +
			  "<br /> Longitude: " + o.Longitude +
			  "<br /> Latitude: " + o.Latitude +
			  "<br /> Altitude: " + o.Altitude + " meters" +
			  "<br /> Time: " + timePretty;
	  }

	  function makeMap() {
		var map = L.map('map');

		map.setView([0, 0], 2);

        window.map = map;
	  }

	  function parseData(result) {
	    var obj = JSON.parse(result);
		obj.sort((a, b) => a.UnixTime - b.UnixTime);
		return obj;
	  }

	  function cleanMap(map) {
		map.eachLayer(function (layer) {
			map.removeLayer(layer);
		});
		if (window.routing) {
			map.removeControl(window.routing);
			window.routing = null;
		}
		L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
			attribution: '© OpenStreetMap contributors'
		}).addTo(map);
	  }
	  function drawDevices(result) {
		var map = window.map;
		var obj = parseData(result);
	    cleanMap(map);
	    drawMarkers(map, obj);
	  }

	  function calculateOpacity(o, i, count) {
		  var weight = (i + 1) / count;
		  return Math.max(0.2, weight);
	  }

	  function drawMarkers(map, obj) {
	  	var count = obj.length;
		var markers = obj.map((o, i) => {
			var latLng = [o.Latitude, o.Longitude];
			var marker = L.marker(latLng, {
				title: o.DeviceID,
				opacity: calculateOpacity(o, i, count),
			});
			marker.addTo(map);

			marker.on('click', function() {
				window.device_id = o.DeviceID;
				$('#historical').prop('disabled', false);
			    L.popup()
				  .setLatLng(latLng)
				  .setContent(makeLabel(o))
				  .openOn(map);
				$('#historical').text('Track ' + o.DeviceID);
			});
			return marker;
		});

		var group = new L.featureGroup(markers);
		map.fitBounds(group.getBounds());
	  }

	  function drawRoute(result) {
		var map = window.map;
		var obj = parseData(result);

	    cleanMap(map);


	    var rawData = obj.map(o => {
	    	return L.latLng(o.Latitude, o.Longitude);
	    });

		var plan = L.Routing.plan(rawData, {
			addWaypoints: false,
			draggableWaypoints: false,
			createMarker: () => false,
		});

		window.routing = L.Routing.control({
			show: false,
			plan: plan,
			lineOptions: {
				styles: [
					{color: 'black', opacity: 0.2, weight: 9},
					{color: 'blue', opacity: 0.8, weight: 6},
					{color: 'blue', opacity: 1, weight: 2},
				],
				addWaypoints: false,
			},
		}).addTo(map);

	    drawMarkers(map, obj);
	  }

	  function loadDevices() {
	    $.ajax({
	      url: "/location/devices",
	      success: drawDevices,
		  failure: alert,
	    });
	  }

	  function getSelectedLocation() {
	  	if (window.device_id === "") {
			alert("select a device first");
			return;
		}
		return window.device_id;
	  }

	  function loadHistoricalData() {
	  	var deviceID = getSelectedLocation()
	    $.ajax({
	      url: "/track/devices/" + deviceID,
	      success: drawRoute,
		  failure: alert,
	    });
	  }

	$(document).ready(function() {
		$('#historical').prop('disabled', true);
		makeMap();
		loadDevices();
	});

    </script>
  </head>

  <body>
    <button onClick=loadDevices()> Load All Devices </button>
    <button disabled id="historical" onClick=loadHistoricalData()> Track Device</button>
    <div id="map" class="map" style="width: 100%; height: 95%"></div>
  </body>
</html>
`
