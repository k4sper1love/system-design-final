package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB() {
	var err error
	db, err = gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	db.AutoMigrate(&User{})
}

func TestRegisterAndLogin(t *testing.T) {
	e := echo.New()
	setupTestDB()
	e.POST("/register", register)
	e.POST("/login", login)

	// Test registration
	regBody := map[string]string{"phone_number": "+77001112233", "password": "testpass"}
	regJSON, _ := json.Marshal(regBody)
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(regJSON))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if err := register(c); err != nil {
		t.Fatalf("register error: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", rec.Code)
	}

	// Test duplicate registration
	rec2 := httptest.NewRecorder()
	c2 := e.NewContext(req, rec2)
	_ = register(c2)
	if rec2.Code != http.StatusConflict {
		t.Errorf("expected 409 for duplicate, got %d", rec2.Code)
	}

	// Test login
	loginBody := map[string]string{"phone_number": "+77001112233", "password": "testpass"}
	loginJSON, _ := json.Marshal(loginBody)
	reqL := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(loginJSON))
	reqL.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	recL := httptest.NewRecorder()
	cL := e.NewContext(reqL, recL)
	if err := login(cL); err != nil {
		t.Fatalf("login error: %v", err)
	}
	if recL.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", recL.Code)
	}

	// Test login with wrong password
	loginBodyBad := map[string]string{"phone_number": "+77001112233", "password": "wrongpass"}
	loginJSONBad, _ := json.Marshal(loginBodyBad)
	reqLB := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(loginJSONBad))
	reqLB.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	recLB := httptest.NewRecorder()
	cLB := e.NewContext(reqLB, recLB)
	_ = login(cLB)
	if recLB.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for wrong password, got %d", recLB.Code)
	}
}
