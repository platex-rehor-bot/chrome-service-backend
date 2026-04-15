# Chrome Service Backend

@AGENTS.md

## Quick Reference

```bash
# Run server
make dev

# Run all tests
make test

# Run specific test package
go test -v ./rest/service/
go test -v ./rest/routes/
go test -v -run TestFunctionName ./rest/service/

# Validate JSON schemas
make validate-schema

# Generate search index
make generate-search-index

# Parse services config
make parse-services

# Database migration
make migrate

# Start local infra (PostgreSQL + Kafka + Unleash)
make infra

# Tear down infra
make clean-all
```

## Code Style

- Follow existing patterns in the codebase
- Route handlers go in `rest/routes/`, business logic in `rest/service/`
- Models embed `models.BaseModel` for standard fields (ID, timestamps, soft delete)
- Use `chi.Router` sub-routers with `MakeXxxRoutes(sub chi.Router)` pattern
- Extract user from context: `user := r.Context().Value(util.USER_CTX_KEY).(models.UserIdentity)`
- Use `database.DB` global for all GORM operations
- Error handling: check and return errors, use `panic()` only for unrecoverable DB errors in handlers
- Logging: use `logrus` (not `log` stdlib) for structured logging
- Config access: `config.Get()` returns the singleton config pointer

## Testing

- Always use `testify/assert` for assertions
- DB-dependent tests require `TestMain` with SQLite setup (see `rest/routes/main_test.go` or `rest/service/dashboardTemplate_test.go` for pattern)
- Create mock data directly with `database.DB.Create()`
- Clean up SQLite DB files in teardown
- Set `cfg.DashboardConfig.TemplatesWD` relative to test file location

## Dependencies

- Do not add dependencies without justification
- Use `go mod tidy` after dependency changes
- Prefer stdlib solutions over external packages when reasonable
