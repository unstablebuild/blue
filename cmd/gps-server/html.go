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

	  function loadData() {
	    $.ajax({
	      url: "/location/devices",
	      success: function(result) {
	    	  var obj = JSON.parse(result);
	    	  var rawData = obj.map(o => {
	    	  	return [o.Latitude, o.Longitude, makeLabel(o)];
	    	  });
	    	  rawData = ([['Lat', 'Long', 'Name']]).concat(rawData);
	    	  console.log(rawData);
              var data = google.visualization.arrayToDataTable(rawData);
              var map = new google.visualization.Map(document.getElementById('map_div'));
              map.draw(data, options);
              console.log('loaded map!');
	      }
	    });
      }

      google.charts.setOnLoadCallback(loadData);
    </script>
  </head>

  <body>
    <div id="map_div" style="width: 100%%; height: 100%%"></div>
  </body>
</html>
`
