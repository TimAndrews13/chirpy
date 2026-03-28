# Chirpy

A Twitter-like REST API built in Go with PostgreSQL. Supports user auth, chirp (post) management, and a premium "Chirpy Red" subscription tier via webhook.

---

## Setup & Installation

### Prerequisites
- Go 1.22+
- PostgreSQL
- [goose](https://github.com/pressly/goose) (for DB migrations)

### Steps

1. **Clone the repo**
   ```bash
   git clone https://github.com/TimAndrews13/chirpy.git
   cd chirpy
   ```

2. **Create your `.env` file** in the project root:
   ```env
   DB_URL=postgres://username:password@localhost:5432/chirpy?sslmode=disable
   PLATFORM=dev
   SECRET=your_jwt_secret_key
   POLKA_KEY=your_polka_webhook_api_key
   ```

3. **Run database migrations**
   ```bash
   goose -dir ./sql/schema postgres "$DB_URL" up
   ```

4. **Install dependencies**
   ```bash
   go mod tidy
   ```

5. **Run the server**
   ```bash
   go run .
   ```
   Server starts on `http://localhost:8080`

---

## Environment Variables

| Variable    | Description                              |
|-------------|------------------------------------------|
| `DB_URL`    | PostgreSQL connection string             |
| `PLATFORM`  | Set to `dev` to enable the reset endpoint |
| `SECRET`    | Secret key used to sign JWTs             |
| `POLKA_KEY` | API key for validating Polka webhooks    |

---

## API Endpoints

### Health

| Method | Endpoint        | Description       | Auth |
|--------|-----------------|-------------------|------|
| GET    | `/api/healthz`  | Readiness check   | None |

---

### Users

| Method | Endpoint     | Description          | Auth         |
|--------|--------------|----------------------|--------------|
| POST   | `/api/users` | Create a new user    | None         |
| PUT    | `/api/users` | Update email/password | Bearer JWT  |

**POST `/api/users`** — Request body:
```json
{
  "email": "user@example.com",
  "password": "yourpassword"
}
```

**PUT `/api/users`** — Request body:
```json
{
  "email": "newemail@example.com",
  "password": "newpassword"
}
```

---

### Auth

| Method | Endpoint       | Description                     | Auth          |
|--------|----------------|---------------------------------|---------------|
| POST   | `/api/login`   | Log in, returns JWT + refresh token | None      |
| POST   | `/api/refresh` | Get a new JWT from refresh token | Bearer refresh token |
| POST   | `/api/revoke`  | Revoke a refresh token           | Bearer refresh token |

**POST `/api/login`** — Request body:
```json
{
  "email": "user@example.com",
  "password": "yourpassword"
}
```

Response includes `token` (JWT, 1hr expiry) and `refresh_token` (60 day expiry).

---

### Chirps

| Method | Endpoint                  | Description            | Auth        |
|--------|---------------------------|------------------------|-------------|
| POST   | `/api/chirps`             | Create a chirp         | Bearer JWT  |
| GET    | `/api/chirps`             | Get all chirps         | None        |
| GET    | `/api/chirps/{chirpID}`   | Get a single chirp     | None        |
| DELETE | `/api/chirps/{chirpID}`   | Delete a chirp         | Bearer JWT  |

**GET `/api/chirps`** — Optional query params:
- `?author_id=<uuid>` — filter by user
- `?sort=asc` or `?sort=desc` — sort by created date (default: `asc`)

**POST `/api/chirps`** — Request body:
```json
{
  "body": "Your chirp text here (max 140 chars)"
}
```

Profanity filter automatically replaces `kerfuffle`, `sharbert`, and `fornax` with `****`.

---

### Webhooks

| Method | Endpoint                  | Description                        | Auth    |
|--------|---------------------------|------------------------------------|---------|
| POST   | `/api/polka/webhooks`     | Upgrade a user to Chirpy Red       | API Key |

Expects `Authorization: ApiKey <POLKA_KEY>` header.

Request body:
```json
{
  "event": "user.upgraded",
  "data": {
    "user_id": "<uuid>"
  }
}
```

Only the `user.upgraded` event triggers an upgrade; all others return `204 No Content`.

---

### Admin

| Method | Endpoint          | Description                          | Auth                  |
|--------|-------------------|--------------------------------------|-----------------------|
| GET    | `/admin/metrics`  | View fileserver hit count            | None                  |
| POST   | `/admin/reset`    | Reset hit count & delete all users   | `PLATFORM=dev` only   |

---

## File Structure

```
chirpy/
├── main.go               # Server setup, routes, apiConfig
├── helpers.go            # respondWithJSON, respondWithError, helperCleanText
├── handlers_admin.go     # Metrics, reset, readiness, middleware
├── handlers_chirps.go    # Chirp CRUD handlers + Chirp struct
├── handlers_users.go     # User creation, login, update + User struct
├── handlers_auth.go      # Refresh, revoke, Polka webhook
└── internal/
    ├── auth/             # JWT, hashing, token helpers
    └── database/         # sqlc-generated DB queries
```