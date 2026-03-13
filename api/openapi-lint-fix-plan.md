# OpenAPI Lint Fix Plan

**File:** `api/openapi.yaml`  
**Linter:** `yaml-schema: swaggerviewer:openapi` (OpenAPI 3.0 schema validation)

## Summary of issues (18 errors)

1. **Missing `description` (16)** – In OpenAPI 3.0, every response object **must** have a `description` field. Several response codes had only `content` and no `description`.
2. **Invalid content shape (2)** – Two responses used `type: array` and `items` directly under the media-type object; in OpenAPI they must be under a `schema` key.

---

## 1. Add missing response descriptions

Add a short `description` for each response that lacks one:

| Location (path / method / code) | Fix |
|--------------------------------|-----|
| `GET /orgs/{orgID}/projects` 200 | `description: OK` |
| `POST /orgs/{orgID}/projects` 201 | `description: Created` |
| `GET /projects/{projectID}` 200 | `description: OK` |
| `GET /projects/{projectID}` 404 | `description: Not found` |
| `GET /projects/{projectID}/tickets` 200 | `description: OK` (and fix schema; see below) |
| `POST /projects/{projectID}/tickets` 201 | `description: Created` |
| `GET /tickets/{ticketID}` 200 | `description: OK` |
| `PATCH /tickets/{ticketID}` 404 | `description: Not found` |
| `GET /projects/{projectID}/reviews` 200 | `description: OK` |
| `DELETE /tickets/{ticketID}/lease` 200 | `description: OK` (response at ~L335) |
| `GET /projects/{projectID}/escalations` 200 | `description: OK` (and fix schema) |
| `GET /projects/{projectID}/escalations` 500 | `description: Internal error` |
| `POST /projects/{projectID}/queue/claim` 200 | `description: OK` |
| `POST /tickets/{ticketID}/lease/renew` 200 | `description: OK` |

---

## 2. Fix array response schema (2 places)

Media-type objects must describe the body with a `schema`. For array responses, put `type: array` and `items` inside that `schema`.

### 2a. `GET /projects/{projectID}/tickets` → 200 (ListTickets)

**Current (invalid):**
```yaml
        "200":
          content:
            application/json:
              type: array
              items:
                $ref: '#/components/schemas/Ticket'
```

**Fixed:**
```yaml
        "200":
          description: OK
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: '#/components/schemas/Ticket'
```

### 2b. `GET /projects/{projectID}/escalations` → 200 (ListEscalations)

**Current (invalid):**
```yaml
        "200":
          content:
            application/json:
              type: array
              items:
                $ref: '#/components/schemas/Escalation'
```

**Fixed:**
```yaml
        "200":
          description: OK
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: '#/components/schemas/Escalation'
```

---

## 3. Verification

- Re-run the OpenAPI / YAML schema linter on `api/openapi.yaml` and confirm zero errors.
- Optionally run a CLI validator (e.g. `npx @redocly/cli lint api/openapi.yaml` or `spectral lint api/openapi.yaml`) if the project adopts one later.

---

## 4. Optional follow-up

- Add a CI step or npm script to lint `api/openapi.yaml` so these rules are enforced on change.
- Consider adopting Redocly or Spectral with a shared config for stricter or consistent OpenAPI quality.
