package main

import (
	"encoding/json"
	"fmt"
	"net/http"

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

func newAPI(store *gps.MultiStore, gmapsAPIKey string) *api {
	router := httprouter.New()

	mapHTML := fmt.Sprintf(template, gmapsAPIKey)
	a := &api{
		router:  router,
		store:   store,
		mapHTML: []byte(mapHTML),
	}

	router.GET("/map", a.serveMap)
	router.GET("/location/devices/:device_id", a.serveDeviceLocation)
	router.GET("/location/devices", a.serveAllDevicesLocation)

	return a
}

func (a *api) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.router.ServeHTTP(w, r)
}
