# New auth routes to register

The following handlers were added to `internal/handlers/auth_handler.go` and corresponding methods in `internal/services/auth_service.go`. Register them in your server (e.g. `cmd/server/main.go` or wherever auth routes are defined) with **auth required** middleware:

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| PUT | `/api/auth/password` | `AuthHandler.ChangePassword` | Change password (body: `current_password`, `new_password`) |
| POST | `/api/auth/deactivate` | `AuthHandler.DeactivateAccount` | Set user `Active` to false |
| DELETE | `/api/auth/me` | `AuthHandler.DeleteAccount` | Delete user record |

Example (Gin):

```go
auth := r.Group("/api/auth")
auth.Use(AuthRequired())
{
  // ... existing routes ...
  auth.PUT("/password", authHandler.ChangePassword)
  auth.POST("/deactivate", authHandler.DeactivateAccount)
  auth.DELETE("/me", authHandler.DeleteAccount)
}
```

Frontend already calls these endpoints; once registered, change password, deactivate, and delete account will work end-to-end.
