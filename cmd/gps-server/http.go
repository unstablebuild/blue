package main

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/ernestrc/blue/document"
	"github.com/ernestrc/blue/gps"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
)

type api struct {
	router  http.Handler
	store   *gps.MultiStore
	mapHTML []byte
}

func (a *api) serveMap(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	w.Write(a.mapHTML)
}

func addAccessControlOriginHeader(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
}

func (a *api) serveAllDevicesLocation(
	w http.ResponseWriter, r *http.Request, ps httprouter.Params,
) {
	ctx := r.Context()
	coords, err := a.store.GetAll(ctx)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	b, err := json.Marshal(coords)
	if err != nil {
		log.Errorf("could not marshal []Coordinates: %v: %v", coords, err)
		return
	}

	addAccessControlOriginHeader(w)
	w.Write(b)
}

func (a *api) serveDeviceLocation(
	w http.ResponseWriter, r *http.Request, ps httprouter.Params,
) {
	deviceID := ps.ByName("device_id")
	if deviceID == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	pos, err := a.store.GetDevice(ctx, deviceID)
	if err != nil {
		if err == document.ErrNotFound {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	b, err := json.Marshal(pos)
	if err != nil {
		log.Errorf("could not marshal Coordinates: %v: %v", pos, err)
		return
	}

	addAccessControlOriginHeader(w)
	w.Write(b)
}

func (a *api) serveDeviceTracks(
	w http.ResponseWriter, r *http.Request, ps httprouter.Params,
) {
	deviceID := ps.ByName("device_id")
	if deviceID == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	it, err := a.store.ListDevice(ctx, deviceID, time.Time{}, time.Now())
	if err != nil {
		if err == document.ErrNotFound {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	w.Write([]byte("["))
	var pos gps.Coordinates
	for it.HasNext() {
		if pos != (gps.Coordinates{}) {
			w.Write([]byte(","))
		}
		err := it.NextTo(&pos)
		if err != nil {
			log.Errorf("could not decode gps.Coordinates: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		b, err := json.Marshal(pos)
		if err != nil {
			log.Errorf("could not marshal Coordinates: %v: %v", pos, err)
			return
		}
		w.Write(b)
	}
	addAccessControlOriginHeader(w)
	w.Write([]byte("]"))

}

func newAPI(store *gps.MultiStore) *api {
	router := httprouter.New()

	a := &api{
		router:  router,
		store:   store,
		mapHTML: []byte(template),
	}

	router.GET("/map", a.serveMap)
	router.GET("/location/devices/:device_id", a.serveDeviceLocation)
	router.GET("/location/devices", a.serveAllDevicesLocation)
	router.GET("/track/devices/:device_id", a.serveDeviceTracks)

	return a
}

func (a *api) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.router.ServeHTTP(w, r)
}
