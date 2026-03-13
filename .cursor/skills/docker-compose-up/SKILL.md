---
name: docker-compose-up
description: Runs docker compose up with build and detach; optionally for a specific service. Use when starting or rebuilding Compose services, when the user says "docker compose up", "bring up", "rebuild server", or "restart" a service.
---

# Docker Compose Up (build + detach)

**Required behavior:** When this skill applies, run the command yourself in the project root. Do not tell the user to run it or paste the command for them to run.

## Command

Run from the project root (where `docker-compose.yml` or `compose.yaml` lives):

```bash
docker compose up --build -d [SERVICE]
```

- **No `SERVICE`**: build and start all services in the background.
- **With `SERVICE`**: build and start only that service (and its dependencies). Use the service name from `docker-compose.yml` (e.g. `server`, `postgres`, `redis`).

## Examples

```bash
# All services
docker compose up --build -d

# One service (and its depends_on)
docker compose up --build -d server

# Infra only
docker compose up --build -d postgres redis
```

## When to use

- User asks to start, bring up, or run Docker Compose.
- User wants to rebuild and restart after code/config changes.
- User names a specific service to start or restart.

## Notes

- Use `--build` so image is rebuilt with current code; omit only if no rebuild is needed.
- Use `-d` for detached (background) mode unless the user asks to follow logs.
- For logs after starting: `docker compose logs -f [SERVICE]` or `docker compose logs [SERVICE]`.
