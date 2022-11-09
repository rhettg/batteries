package main

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"runtime/debug"
	"time"

	"go.uber.org/zap"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rhettg/batteries/yakapi/internal/ci"
	mw "github.com/rhettg/batteries/yakapi/internal/mw"
	"tailscale.com/client/tailscale"
)

var (
	//go:embed assets/*
	assets embed.FS
)

func home(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Location", "/v1")
	w.WriteHeader(http.StatusTemporaryRedirect)
}

func eyes(w http.ResponseWriter, r *http.Request) {
	content, err := assets.ReadFile("assets/index.html")
	if err != nil {
		errorResponse(w, err, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(content)
}

type resource struct {
	Name string `json:"name"`
	Ref  string `json:"ref"`
}

var startTime time.Time

func init() {
	startTime = time.Now()
}

var (
	opsProcessed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "yakapi_processed_ops_total",
		Help: "The total number of processed requests",
	})
)

func errorResponse(w http.ResponseWriter, respErr error, statusCode int) error {
	resp := struct {
		Error string `json:"error"`
	}{Error: respErr.Error()}

	return sendResponse(w, resp, statusCode)
}

func sendResponse(w http.ResponseWriter, resp interface{}, statusCode int) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	err := json.NewEncoder(w).Encode(resp)
	if err != nil {
		return err
	}

	return nil
}

func me(w http.ResponseWriter, r *http.Request) {
	whois, err := tailscale.WhoIs(r.Context(), r.RemoteAddr)
	if err != nil {
		errorResponse(w, errors.New("unknown"), http.StatusInternalServerError)
		log.Errorw("whois failure", "error", err)
		return
	}

	resp := struct {
		Name   string `json:"name"`
		Login  string `json:"login"`
		Device string `json:"device"`
	}{
		Name:   whois.UserProfile.DisplayName,
		Login:  whois.UserProfile.LoginName,
		Device: whois.Node.Hostinfo.Hostname(),
	}

	err = sendResponse(w, &resp, http.StatusOK)
	if err != nil {
		log.Errorw("error sending response", "err", err)
		return
	}
}

func handleCI(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		errorResponse(w, errors.New("POST required"), http.StatusMethodNotAllowed)
		return
	}

	if r.Header.Get("content-type") != "application/json" {
		errorResponse(w, errors.New("application/json required"), http.StatusUnsupportedMediaType)
		return
	}

	req := struct {
		Command string `json:"command"`
	}{}

	err := json.NewDecoder(r.Body).Decode(&req)
	defer r.Body.Close()

	if err != nil {
		log.Errorw("failed parsing body", "error", err)
		errorResponse(w, errors.New("failed parsing body"), http.StatusBadRequest)
		return
	}

	err = ci.Accept(r.Context(), req.Command)
	if err != nil {
		log.Errorw("failed accepting ci command", "error", err)
		errorResponse(w, err, http.StatusBadRequest)
		return
	}

	resp := struct {
		Result string `json:"result"`
	}{
		Result: "ok",
	}

	err = sendResponse(w, resp, http.StatusAccepted)
	if err != nil {
		log.Errorw("error sending response", "error", err)
		return
	}
}

func handleCIPush(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		errorResponse(w, errors.New("POST required"), http.StatusMethodNotAllowed)
		return
	}

	if r.Header.Get("Upgrade") != "rover" {
		errorResponse(w, errors.New("not an upgrade"), http.StatusMethodNotAllowed)
		return
	}

	w.WriteHeader(http.StatusSwitchingProtocols)

	fmt.Println("upgrade complete")

	// hijack the connection
	conn, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		errorResponse(w, errors.New("hijack failed"), http.StatusMethodNotAllowed)
		return
	}

	fmt.Println("hijack complete")

	for n := 0; n < 3; n++ {
		conn.Write([]byte("rt 30\n"))
		time.Sleep(1 * time.Second)
	}

	conn.Close()

	fmt.Println("closed")
}

func handleCamCapture(w http.ResponseWriter, r *http.Request) {
	captureFile := os.Getenv("YAKAPI_CAM_CAPTURE_PATH")
	if captureFile == "" {
		err := errors.New("YAKAPI_CAM_CAPTURE_PATH not configured")
		errorResponse(w, err, http.StatusInternalServerError)
		return
	}

	content, err := ioutil.ReadFile(captureFile)
	if err != nil {
		errorResponse(w, err, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/jpeg")
	w.WriteHeader(http.StatusOK)
	w.Write(content)
}

func homev1(w http.ResponseWriter, r *http.Request) {
	opsProcessed.Inc()

	resp := struct {
		Name      string     `json:"name"`
		UpTime    int64      `json:"uptime"`
		Resources []resource `json:"resources"`
	}{
		Name:   "YakAPI Server",
		UpTime: int64(time.Since(startTime).Seconds()),
		Resources: []resource{
			{Name: "metrics", Ref: "/metrics"},
			{Name: "ci", Ref: "/v1/ci"},
			{Name: "cam", Ref: "/v1/cam/capture"},
		},
	}

	name := os.Getenv("YAKAPI_NAME")
	if name != "" {
		resp.Name = name
	}

	project := os.Getenv("YAKAPI_PROJECT")
	if project != "" {
		resp.Resources = append(resp.Resources, resource{Name: "project", Ref: project})
	}

	operator := os.Getenv("YAKAPI_OPERATOR")
	if operator != "" {
		resp.Resources = append(resp.Resources, resource{Name: "operator", Ref: operator})
	}

	err := sendResponse(w, resp, http.StatusOK)
	if err != nil {
		log.Errorw("error sending response", "error", err)
		return
	}
}

var log *zap.SugaredLogger

func main() {
	logger, _ := zap.NewDevelopment()
	defer logger.Sync() // flushes buffer, if any
	log = logger.Sugar()

	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "yakapi_uptime_seconds",
		Help: "The uptime of the yakapi service",
	}, func() float64 {
		return float64(time.Since(startTime).Seconds())
	})

	logmw := mw.New(logger)

	http.Handle("/", logmw(http.HandlerFunc(home)))
	http.Handle("/v1", logmw(http.HandlerFunc(homev1)))
	http.Handle("/v1/me", logmw(http.HandlerFunc(me)))
	http.Handle("/v1/ci", logmw(http.HandlerFunc(handleCI)))
	http.Handle("/v1/cam/capture", logmw(http.HandlerFunc(handleCamCapture)))
	http.Handle("/metrics", logmw(promhttp.Handler()))
	http.Handle("/eyes", logmw(http.HandlerFunc(eyes)))
	http.Handle("/v1/ci_push", http.HandlerFunc(handleCIPush))

	port := os.Getenv("YAKAPI_PORT")
	if port == "" {
		port = "8080"
	}

	info, ok := debug.ReadBuildInfo()
	if !ok {
		log.Errorw("failed loading build info")
	}

	revision := "unknown"
	for _, s := range info.Settings {
		if s.Key == "vcs.revision" {
			revision = s.Value
			break
		}
	}

	log.Infow("starting", "version", "1.0.0", "port", port, "build", revision)
	err := http.ListenAndServe(fmt.Sprintf(":%s", port), nil)
	if err != nil {
		log.Errorw("error from ListenAndServer", "error", err)
	}
}
