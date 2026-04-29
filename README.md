# Logistics Routing Web Application

University course project implemented in Go + vanilla HTML/CSS/JavaScript, containerized with Docker.

## Project Structure

- `main.go` - backend HTTP server, REST endpoints, route/fuel/cargo logic, session logging, JSON persistence.
- `static/index.html` - main UI with truck/destination selection, progress, output logs, and modal forms.
- `static/app.js` - frontend logic for API calls, route preview/send, add-by-material, and modal actions.
- `static/styles.css` - basic page and modal styling.
- `data/state.json` - persistent trucks and destinations state (auto-created on first start).
- `logs/session-*.txt` - session logs written by backend.
- `Dockerfile` - builds and runs the Go web app.
- `docker-compose.yml` - orchestrates app service and mounts persistent data/logs.
- `Makefile` - utility commands for Docker and local run.

## How to Run

### 1) Run in Docker (recommended)

```powershell
make up
```

Open: [http://localhost:8080](http://localhost:8080)

Stop containers:

```powershell
make down
```

View container logs:

```powershell
make logs
```

### 2) Run locally (without Docker)

```powershell
go run main.go
```

Open: [http://localhost:8080](http://localhost:8080)

## Implemented Features by Phase

- **Phase 1 (9.1):** Initial 2 trucks + 4 destinations, route calculation using sorted distance from base 0, return-to-base distance included, fuel validation, output log area, and backend session log `.txt` file.
- **Phase 2 (9.2):** Material types A/B/C/D, cargo compatibility rules, live fuel `<progress>` preview, add-by-material optimization, and persistent state load/save via JSON.
- **Phase 3 (9.3):** Error popups (modals), add truck modal (with validations and unique name), add destination modal, truck status modal with progress and route/error summary, and append persistence for new entities.
