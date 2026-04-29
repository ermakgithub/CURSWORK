package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	stateFilePath = "data/state.json"
	logsDir       = "logs"
)

type Material string

const (
	MaterialA Material = "A"
	MaterialB Material = "B"
	MaterialC Material = "C"
	MaterialD Material = "D"
)

type Destination struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Distance float64  `json:"distance"`
	Material Material `json:"material"`
}

type Truck struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	FuelCapacity float64 `json:"fuelCapacity"`
}

type AppState struct {
	Trucks       []Truck       `json:"trucks"`
	Destinations []Destination `json:"destinations"`
}

type App struct {
	mu         sync.Mutex
	state      AppState
	sessionLog *os.File
}

type routeRequest struct {
	TruckID         string   `json:"truckId"`
	DestinationIDs  []string `json:"destinationIds"`
	AllowEmptyRoute bool     `json:"allowEmptyRoute"`
}

type addMaterialRequest struct {
	TruckID    string   `json:"truckId"`
	CurrentIDs []string `json:"currentDestinationIds"`
	Material   Material `json:"material"`
}

type addTruckRequest struct {
	Name         string  `json:"name"`
	FuelCapacity float64 `json:"fuelCapacity"`
}

type addDestinationRequest struct {
	Name     string   `json:"name"`
	Distance float64  `json:"distance"`
	Material Material `json:"material"`
}

type routeResponse struct {
	OK             bool     `json:"ok"`
	Message        string   `json:"message"`
	RouteNames     []string `json:"routeNames,omitempty"`
	RouteIDs       []string `json:"routeIds,omitempty"`
	TotalDistance  float64  `json:"totalDistance,omitempty"`
	FuelPercentage float64  `json:"fuelPercentage,omitempty"`
}

func main() {
	app, err := newApp()
	if err != nil {
		log.Fatalf("failed to start app: %v", err)
	}
	defer func() {
		if app.sessionLog != nil {
			_ = app.sessionLog.Close()
		}
	}()

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir("static")))
	mux.HandleFunc("/api/state", app.handleGetState)
	mux.HandleFunc("/api/route/preview", app.handlePreviewRoute)
	mux.HandleFunc("/api/route/send", app.handleSendRoute)
	mux.HandleFunc("/api/add-by-material", app.handleAddByMaterial)
	mux.HandleFunc("/api/trucks", app.handleAddTruck)
	mux.HandleFunc("/api/destinations", app.handleAddDestination)
	mux.HandleFunc("/api/truck-status", app.handleTruckStatus)

	addr := ":8080"
	log.Printf("server running at http://localhost%s", addr)
	if err := http.ListenAndServe(addr, withCORS(mux)); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func newApp() (*App, error) {
	if err := os.MkdirAll("data", 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		return nil, err
	}

	state, err := loadOrCreateState(stateFilePath)
	if err != nil {
		return nil, err
	}

	logName := fmt.Sprintf("session-%s.txt", time.Now().Format("20060102-150405"))
	logPath := filepath.Join(logsDir, logName)
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}

	return &App{
		state:      state,
		sessionLog: logFile,
	}, nil
}

func loadOrCreateState(path string) (AppState, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		var st AppState
		if e := json.Unmarshal(data, &st); e != nil {
			return AppState{}, e
		}
		return st, nil
	}

	if !errors.Is(err, os.ErrNotExist) {
		return AppState{}, err
	}

	st := AppState{
		Trucks: []Truck{
			{ID: "TRK-001", Name: "Alpha", FuelCapacity: 120},
			{ID: "TRK-002", Name: "Beta", FuelCapacity: 80},
		},
		Destinations: []Destination{
			{ID: "DST-001", Name: "North Gate", Distance: 15, Material: MaterialA},
			{ID: "DST-002", Name: "South Hub", Distance: -12, Material: MaterialB},
			{ID: "DST-003", Name: "West Point", Distance: 23, Material: MaterialC},
			{ID: "DST-004", Name: "East Depot", Distance: -30, Material: MaterialD},
		},
	}

	if err := saveState(path, st); err != nil {
		return AppState{}, err
	}
	return st, nil
}

func saveState(path string, st AppState) error {
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func (a *App) logSession(msg string) {
	if a.sessionLog == nil {
		return
	}
	_, _ = a.sessionLog.WriteString(time.Now().Format(time.RFC3339) + " " + msg + "\n")
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func parseJSON(r *http.Request, out any) error {
	return json.NewDecoder(r.Body).Decode(out)
}

func materialValid(m Material) bool {
	return m == MaterialA || m == MaterialB || m == MaterialC || m == MaterialD
}

func totalRouteDistance(destinations []Destination) float64 {
	if len(destinations) == 0 {
		return 0
	}
	sorted := make([]Destination, len(destinations))
	copy(sorted, destinations)
	sort.Slice(sorted, func(i, j int) bool {
		return abs(sorted[i].Distance) < abs(sorted[j].Distance)
	})

	var total float64
	current := 0.0
	for _, d := range sorted {
		total += abs(current - d.Distance)
		current = d.Distance
	}
	total += abs(current)
	return total
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func validateCargo(destinations []Destination) error {
	if len(destinations) == 0 {
		return nil
	}

	hasB := false
	hasC := false
	hasD := false

	for _, d := range destinations {
		switch d.Material {
		case MaterialB:
			hasB = true
		case MaterialC:
			hasC = true
		case MaterialD:
			hasD = true
		case MaterialA:
		default:
			return fmt.Errorf("unknown material type on destination %s", d.Name)
		}
	}

	if hasD && len(destinations) > 1 {
		return errors.New("material D must be transported alone")
	}
	if hasB && hasC {
		return errors.New("material B cannot be transported together with material C")
	}
	return nil
}

func (a *App) getTruckByID(id string) (Truck, bool) {
	for _, t := range a.state.Trucks {
		if t.ID == id {
			return t, true
		}
	}
	return Truck{}, false
}

func (a *App) destinationsByIDs(ids []string) ([]Destination, error) {
	destMap := make(map[string]Destination, len(a.state.Destinations))
	for _, d := range a.state.Destinations {
		destMap[d.ID] = d
	}

	result := make([]Destination, 0, len(ids))
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			continue
		}
		d, ok := destMap[id]
		if !ok {
			return nil, fmt.Errorf("destination with id %s not found", id)
		}
		seen[id] = true
		result = append(result, d)
	}
	return result, nil
}

func (a *App) calculateRoute(truckID string, destinationIDs []string, allowEmpty bool) (routeResponse, error) {
	truck, ok := a.getTruckByID(truckID)
	if !ok {
		return routeResponse{}, errors.New("selected truck was not found")
	}
	destinations, err := a.destinationsByIDs(destinationIDs)
	if err != nil {
		return routeResponse{}, err
	}
	if len(destinations) == 0 && !allowEmpty {
		return routeResponse{}, errors.New("select at least one destination")
	}
	if err := validateCargo(destinations); err != nil {
		return routeResponse{}, err
	}

	sorted := make([]Destination, len(destinations))
	copy(sorted, destinations)
	sort.Slice(sorted, func(i, j int) bool {
		return abs(sorted[i].Distance) < abs(sorted[j].Distance)
	})

	total := totalRouteDistance(sorted)
	fuelPct := 0.0
	if truck.FuelCapacity > 0 {
		fuelPct = (total / truck.FuelCapacity) * 100
	}
	if total > truck.FuelCapacity {
		return routeResponse{}, fmt.Errorf("route requires %.2f fuel, but truck %s has %.2f", total, truck.Name, truck.FuelCapacity)
	}

	routeNames := make([]string, 0, len(sorted))
	routeIDs := make([]string, 0, len(sorted))
	for _, d := range sorted {
		routeNames = append(routeNames, d.Name)
		routeIDs = append(routeIDs, d.ID)
	}

	return routeResponse{
		OK:             true,
		Message:        fmt.Sprintf("Optimal route: 0 -> %s -> 0 | Total distance: %.2f", strings.Join(routeNames, " -> "), total),
		RouteNames:     routeNames,
		RouteIDs:       routeIDs,
		TotalDistance:  total,
		FuelPercentage: fuelPct,
	}, nil
}

func (a *App) handleGetState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	writeJSON(w, http.StatusOK, a.state)
}

func (a *App) handlePreviewRoute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req routeRequest
	if err := parseJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request payload"})
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	resp, err := a.calculateRoute(req.TruckID, req.DestinationIDs, true)
	if err != nil {
		truck, ok := a.getTruckByID(req.TruckID)
		if !ok {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "selected truck was not found"})
			return
		}
		dests, derr := a.destinationsByIDs(req.DestinationIDs)
		if derr != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": derr.Error()})
			return
		}
		total := totalRouteDistance(dests)
		pct := 0.0
		if truck.FuelCapacity > 0 {
			pct = (total / truck.FuelCapacity) * 100
		}
		writeJSON(w, http.StatusOK, routeResponse{
			OK:             false,
			Message:        err.Error(),
			TotalDistance:  total,
			FuelPercentage: pct,
		})
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func (a *App) handleSendRoute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var req routeRequest
	if err := parseJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request payload"})
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	resp, err := a.calculateRoute(req.TruckID, req.DestinationIDs, false)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	a.logSession(resp.Message)
	writeJSON(w, http.StatusOK, resp)
}

func (a *App) handleAddByMaterial(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var req addMaterialRequest
	if err := parseJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request payload"})
		return
	}
	if !materialValid(req.Material) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid material"})
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	truck, ok := a.getTruckByID(req.TruckID)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "selected truck was not found"})
		return
	}

	current, err := a.destinationsByIDs(req.CurrentIDs)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := validateCargo(current); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "current selection is invalid: " + err.Error()})
		return
	}

	currentSet := map[string]bool{}
	for _, d := range current {
		currentSet[d.ID] = true
	}

	candidates := make([]Destination, 0)
	for _, d := range a.state.Destinations {
		if d.Material == req.Material && !currentSet[d.ID] {
			candidates = append(candidates, d)
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		return abs(candidates[i].Distance) < abs(candidates[j].Distance)
	})

	selected := append([]Destination{}, current...)
	added := make([]Destination, 0)
	for _, c := range candidates {
		try := append([]Destination{}, selected...)
		try = append(try, c)
		if err := validateCargo(try); err != nil {
			continue
		}
		if totalRouteDistance(try) > truck.FuelCapacity {
			continue
		}
		selected = try
		added = append(added, c)
	}

	finalIDs := make([]string, 0, len(selected))
	for _, d := range selected {
		finalIDs = append(finalIDs, d.ID)
	}

	msg := "No additional destinations could be added."
	if len(added) > 0 {
		names := make([]string, 0, len(added))
		for _, d := range added {
			names = append(names, d.Name)
		}
		msg = "Added destinations: " + strings.Join(names, ", ")
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":             true,
		"message":        msg,
		"destinationIds": finalIDs,
	})
}

func (a *App) handleAddTruck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var req addTruckRequest
	if err := parseJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request payload"})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "truck name is required"})
		return
	}
	if req.FuelCapacity <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "fuel capacity must be positive"})
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	for _, t := range a.state.Trucks {
		if strings.EqualFold(t.Name, req.Name) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "truck name must be unique"})
			return
		}
	}

	newTruck := Truck{
		ID:           "TRK-" + strconv.FormatInt(time.Now().UnixNano(), 10),
		Name:         req.Name,
		FuelCapacity: req.FuelCapacity,
	}
	a.state.Trucks = append(a.state.Trucks, newTruck)
	if err := saveState(stateFilePath, a.state); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to persist truck"})
		return
	}
	writeJSON(w, http.StatusCreated, newTruck)
}

func (a *App) handleAddDestination(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var req addDestinationRequest
	if err := parseJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request payload"})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "destination name is required"})
		return
	}
	if !materialValid(req.Material) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid material type"})
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	newDest := Destination{
		ID:       "DST-" + strconv.FormatInt(time.Now().UnixNano(), 10),
		Name:     req.Name,
		Distance: req.Distance,
		Material: req.Material,
	}
	a.state.Destinations = append(a.state.Destinations, newDest)
	if err := saveState(stateFilePath, a.state); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to persist destination"})
		return
	}
	writeJSON(w, http.StatusCreated, newDest)
}

func (a *App) handleTruckStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var req struct {
		TruckID string `json:"truckId"`
	}
	if err := parseJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request payload"})
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	truck, ok := a.getTruckByID(req.TruckID)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "selected truck was not found"})
		return
	}

	allIDs := make([]string, 0, len(a.state.Destinations))
	for _, d := range a.state.Destinations {
		allIDs = append(allIDs, d.ID)
	}
	resp, err := a.calculateRoute(truck.ID, allIDs, true)
	if err != nil {
		dests, _ := a.destinationsByIDs(allIDs)
		total := totalRouteDistance(dests)
		pct := 0.0
		if truck.FuelCapacity > 0 {
			pct = (total / truck.FuelCapacity) * 100
		}
		writeJSON(w, http.StatusOK, routeResponse{
			OK:             false,
			Message:        err.Error(),
			TotalDistance:  total,
			FuelPercentage: pct,
		})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}
