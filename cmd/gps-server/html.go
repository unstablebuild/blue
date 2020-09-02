package main

const template = `
<html>
  <head>
	<script src="https://cdnjs.cloudflare.com/ajax/libs/jquery/3.5.1/jquery.min.js" integrity="sha512-bLT0Qm9VnAYZDflyKcBaQ2gg0hSYNQrJ8RilYldYQ1FxQYoCLtUjuuRuZo+fjqhx/qtq/1itJ0C2ejDxltZVFg==" crossorigin="anonymous"></script>
	<script src="https://cdnjs.cloudflare.com/ajax/libs/moment.js/2.27.0/moment.min.js" integrity="sha512-rmZcZsyhe0/MAjquhTgiUcb4d9knaFc7b5xAfju483gbEXTkeJRUMIPk6s3ySZMYUHEcjKbjLjyddGWMrNEvZg==" crossorigin="anonymous"></script>
    <script type="text/javascript" src="https://www.gstatic.com/charts/loader.js"></script>
    <script type="text/javascript">
      google.charts.load("current", {
        "packages":["map"],
		'mapsApiKey': '%s'
      });

      var options = {
        showTooltip: true,
        showInfoWindow: true,
        useMapTypeControl: true,
		enableScrollWheel: true,
	  }

	  function makeLabel(o) {
		  var lastSeen = moment.unix(o.UnixTime).fromNow();
		  return o.DeviceID + " " + lastSeen;
	  }

	  function parseDrawResults(result) {

		$('#historical').prop('disabled', true);

	    var obj = JSON.parse(result);
	    var rawData = obj.map(o => {
	    	return [o.Latitude, o.Longitude, makeLabel(o)];
	    });
	    rawData = ([['Lat', 'Long', 'Name']]).concat(rawData);
        var data = google.visualization.arrayToDataTable(rawData);
        var map = new google.visualization.Map(document.getElementById('map_div'));
        map.draw(data, options);

		google.visualization.events.addListener(map, 'select', function() {
			$('#historical').prop('disabled', function(i, v) { return !v; });
		});

        window.google_map = map;
        window.raw_data = obj;
	  }

	  function loadDevicesData() {
	    $.ajax({
	      url: "/location/devices",
	      success: parseDrawResults,
		  failure: alert,
	    });
	  }

	  function getSelectedLocation() {
		  if (!window.google_map) {
		  	  return "";
		  }

		  var selection = window.google_map.getSelection();
		  if (selection.length === 0) {
		  	  return "";
		  }
		  var idx = selection[0].row;

		  if (!window.raw_data || window.raw_data.length < idx+1) {
		  	  console.error("raw_data is shorter than selected row?");
		  	  return "";
		  }

		  var position = window.raw_data[idx];

		  return position.DeviceID;
	  }

	  function loadHistoricalData() {
	  	var deviceID = getSelectedLocation()
	  	if (deviceID === "") {
			alert("select a device first");
			return;
		}
	    $.ajax({
	      url: "/track/devices/" + deviceID,
	      success: parseDrawResults,
		  failure: alert,
	    });
	  }

      google.charts.setOnLoadCallback(loadDevicesData);
    </script>
  </head>

  <body>
    <button onClick=loadDevicesData()> Load All Devices </button>
    <button disabled id="historical" onClick=loadHistoricalData()> Track Device</button>
    <div id="map_div" style="width: 100%%; height: 100%%"></div>
  </body>
</html>
`
