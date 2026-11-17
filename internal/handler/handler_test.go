package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"pr5_minenko/internal/auth"
	"pr5_minenko/internal/config"
	"pr5_minenko/internal/model"
)

const testSecret = "test-secret"

func newTestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger:         logger.Default.LogMode(logger.Silent),
		TranslateError: true,
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.Incident{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return NewRouter(db, config.Config{JWTSecret: testSecret, JWTTTL: time.Hour})
}

func do(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	var buf *bytes.Reader
	if body != "" {
		buf = bytes.NewReader([]byte(body))
	} else {
		buf = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, buf)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func expect(t *testing.T, w *httptest.ResponseRecorder, code int) {
	t.Helper()
	if w.Code != code {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, code, w.Body.String())
	}
}

func TestIncidentLifecycle(t *testing.T) {
	r := newTestRouter(t)

	w := do(r, http.MethodPost, "/api/v1/incidents",
		`{"title":"Overheating on Gateway-01","description":"Core temperature reached 85C","priority":"critical"}`)
	expect(t, w, http.StatusCreated)
	var created model.Incident
	json.Unmarshal(w.Body.Bytes(), &created)
	if created.ID == 0 || created.Status != "open" || created.Priority != "critical" {
		t.Fatalf("unexpected incident: %+v", created)
	}

	w = do(r, http.MethodGet, "/api/v1/incidents", "")
	expect(t, w, http.StatusOK)
	var list []model.Incident
	json.Unmarshal(w.Body.Bytes(), &list)
	if len(list) != 1 {
		t.Fatalf("list len = %d, want 1", len(list))
	}

	path := fmt.Sprintf("/api/v1/incidents/%d", created.ID)
	expect(t, do(r, http.MethodGet, path, ""), http.StatusOK)

	w = do(r, http.MethodPatch, path+"/status", `{"status":"in_progress"}`)
	expect(t, w, http.StatusOK)
	var updated model.Incident
	json.Unmarshal(w.Body.Bytes(), &updated)
	if updated.Status != "in_progress" {
		t.Fatalf("status = %q, want in_progress", updated.Status)
	}

	w = do(r, http.MethodDelete, path, "")
	expect(t, w, http.StatusNoContent)
	if w.Body.Len() != 0 {
		t.Fatalf("204 must have empty body, got %q", w.Body.String())
	}
	expect(t, do(r, http.MethodGet, path, ""), http.StatusNotFound)
	expect(t, do(r, http.MethodDelete, path, ""), http.StatusNotFound)
}

func TestEmptyListIsArray(t *testing.T) {
	r := newTestRouter(t)
	w := do(r, http.MethodGet, "/api/v1/incidents", "")
	expect(t, w, http.StatusOK)
	if strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("body = %s, want []", w.Body.String())
	}
}

func TestIncidentValidation(t *testing.T) {
	r := newTestRouter(t)
	cases := map[string]struct{ body, field string }{
		"invalid priority": {`{"title":"Disk failure","description":"RAID degraded","priority":"urgent"}`, "priority"},
		"short title":      {`{"title":"ab","description":"x","priority":"low"}`, "title"},
		"missing desc":     {`{"title":"Disk failure","priority":"low"}`, "description"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			w := do(r, http.MethodPost, "/api/v1/incidents", tc.body)
			expect(t, w, http.StatusBadRequest)
			var resp struct {
				Fields map[string]string `json:"fields"`
			}
			json.Unmarshal(w.Body.Bytes(), &resp)
			if _, ok := resp.Fields[tc.field]; !ok {
				t.Fatalf("expected error for field %q, got %s", tc.field, w.Body.String())
			}
		})
	}

	expect(t, do(r, http.MethodPost, "/api/v1/incidents", `{bad json`), http.StatusBadRequest)
	expect(t, do(r, http.MethodGet, "/api/v1/incidents/abc", ""), http.StatusBadRequest)
	expect(t, do(r, http.MethodGet, "/api/v1/incidents/999", ""), http.StatusNotFound)
}

func TestUpdateStatusValidation(t *testing.T) {
	r := newTestRouter(t)
	do(r, http.MethodPost, "/api/v1/incidents", `{"title":"Network down","description":"x","priority":"high"}`)
	expect(t, do(r, http.MethodPatch, "/api/v1/incidents/1/status", `{"status":"done"}`), http.StatusBadRequest)
	expect(t, do(r, http.MethodPatch, "/api/v1/incidents/42/status", `{"status":"closed"}`), http.StatusNotFound)
}

func TestRegisterAndLogin(t *testing.T) {
	r := newTestRouter(t)
	creds := `{"email":"engineer@company.internal","password":"SuperSecretPassword123"}`

	w := do(r, http.MethodPost, "/api/v1/auth/register", creds)
	expect(t, w, http.StatusCreated)
	if strings.Contains(w.Body.String(), "password") || strings.Contains(w.Body.String(), "$2a$") {
		t.Fatalf("password hash leaked: %s", w.Body.String())
	}

	expect(t, do(r, http.MethodPost, "/api/v1/auth/register",
		`{"email":"ENGINEER@company.internal","password":"another1"}`), http.StatusConflict)

	w = do(r, http.MethodPost, "/api/v1/auth/login", creds)
	expect(t, w, http.StatusOK)
	var resp struct {
		Token string `json:"token"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	claims, err := auth.ParseToken(resp.Token, testSecret)
	if err != nil {
		t.Fatalf("token invalid: %v", err)
	}
	if claims.UserID != 1 {
		t.Fatalf("userID = %d, want 1", claims.UserID)
	}
	if _, err := auth.ParseToken(resp.Token, "wrong-secret"); err == nil {
		t.Fatal("token must not validate with a different secret")
	}

	expect(t, do(r, http.MethodPost, "/api/v1/auth/login",
		`{"email":"engineer@company.internal","password":"wrong-password"}`), http.StatusUnauthorized)
	expect(t, do(r, http.MethodPost, "/api/v1/auth/login",
		`{"email":"nobody@company.internal","password":"whatever"}`), http.StatusUnauthorized)
}

func TestRegisterValidation(t *testing.T) {
	r := newTestRouter(t)
	expect(t, do(r, http.MethodPost, "/api/v1/auth/register", `{"email":"not-an-email","password":"123456"}`), http.StatusBadRequest)
	expect(t, do(r, http.MethodPost, "/api/v1/auth/register", `{"email":"a@b.com","password":"123"}`), http.StatusBadRequest)
}
