# AppDeck

Monorepo for the AppDeck platform:

| Directory         | What it is                                          |
|--------------------|------------------------------------------------------|
| `backend/`         | Go (Gin + GORM) API server, MySQL-backed             |
| `frontend-svelte/` | Admin dashboard (SvelteKit 2 / Svelte 5)              |
| `client-android/`  | Android TV/kiosk client (Kotlin)                      |

## 1. MySQL (via Docker)

Run a MySQL 8.0 container for local development:

```bash
docker run -d \
  --name appdeck \
  --restart unless-stopped \
  -p 127.0.0.1:3306:3306 \
  -e MYSQL_ROOT_PASSWORD=root123 \
  -e MYSQL_DATABASE=appdeck \
  -e MYSQL_CHARACTER_SET_SERVER=utf8mb4 \
  -v mysql_data:/var/lib/mysql \
  mysql:8.0
```

This creates:

- Container `appdeck`, exposed on `127.0.0.1:3306`
- Database `appdeck` (utf8mb4), root password `root123`
- Persistent volume `mysql_data`

Check it's up:

```bash
docker ps
docker logs -f appdeck
```

Stop it (keeps data) / remove it and wipe data:

```bash
docker stop appdeck
docker rm -f appdeck && docker volume rm mysql_data   # also deletes all data
```

Load the schema once the container is healthy:

```bash
mysql -h 127.0.0.1 -P 3306 -u root -proot123 appdeck < backend/database/schema.sql
```

(Or skip this and let the backend auto-migrate — see below.)

## 2. Backend (`backend/`)

Requires Go 1.21+.

```bash
cd backend
go mod download
cp env.example .env
```

Edit `.env` to match the Docker MySQL above, e.g.:

```
DB_HOST=127.0.0.1
DB_PORT=3306
DB_USER=root
DB_PASSWORD=root123
DB_NAME=appdeck
AUTO_MIGRATE=true
```

Run:

```bash
go run main.go
```

Server listens on `http://localhost:8080` (`SERVER_PORT` in `.env`).

Build a binary:

```bash
go build -o appdeck-server main.go
```

More detail (API endpoints, project layout, production deploy with systemd/nginx): `backend/README_BACKEND.md`.

## 3. Admin frontend (`frontend-svelte/`)

Requires Node.js with npm.

```bash
cd frontend-svelte
npm install
npm run dev
```

Other scripts:

```bash
npm run build      # production build
npm run preview    # preview the production build
npm run check      # svelte-check type checking
```

The dev server proxies/calls the backend API — make sure `backend` is running first (default `http://localhost:8080`).

## 4. Android client (`client-android/`)

Requires JDK 17+, Android SDK (API 34), and a Gradle wrapper (none is checked in — generate it once with `gradle wrapper --gradle-version 8.5`).

```bash
cd client-android
./gradlew assembleDebug
# APK: app/build/outputs/apk/debug/app-debug.apk

adb install -r app/build/outputs/apk/debug/app-debug.apk
```

Full prerequisites, signing, and device setup notes: `client-android/BUILD.md`.

## Typical local dev order

1. `docker run ...` (MySQL, see above)
2. `cd backend && go run main.go` (API on :8080)
3. `cd frontend-svelte && npm run dev` (admin UI)
4. `cd client-android && ./gradlew installDebug` (device client, points at the backend)
