package jsonapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/a-h/jsonapi"
	"github.com/a-h/respond"
	"github.com/google/go-cmp/cmp"
)

type itemsGetResponse struct {
	Items []string `json:"items"`
}

var expectedItemsGetResponse = itemsGetResponse{
	Items: []string{"item1", "item2"},
}

func createTestRoutes() *http.ServeMux {
	routes := http.NewServeMux()
	routes.HandleFunc("/items/get/ok", func(w http.ResponseWriter, r *http.Request) {
		respond.WithJSON(w, expectedItemsGetResponse, http.StatusOK)
	})
	routes.HandleFunc("/items/get/404", func(w http.ResponseWriter, r *http.Request) {
		respond.WithError(w, "Not found", http.StatusNotFound)
	})
	routes.HandleFunc("/items/get/500", func(w http.ResponseWriter, r *http.Request) {
		respond.WithError(w, "Internal server error", http.StatusInternalServerError)
	})
	routes.HandleFunc("/items/post/ok", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			respond.WithError(w, "Expected application/json content type", http.StatusBadRequest)
			return
		}
		var m map[string]any
		if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
			respond.WithError(w, err.Error(), http.StatusBadRequest)
			return
		}
		respond.WithJSON(w, m, http.StatusCreated)
	})
	routes.HandleFunc("/items/post/404", func(w http.ResponseWriter, r *http.Request) {
		respond.WithError(w, "Not found", http.StatusNotFound)
	})
	routes.HandleFunc("/items/post/500", func(w http.ResponseWriter, r *http.Request) {
		respond.WithError(w, "Internal server error", http.StatusInternalServerError)
	})
	routes.HandleFunc("/auth/items/get/ok", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer abc" {
			respond.WithError(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		respond.WithJSON(w, expectedItemsGetResponse, http.StatusOK)
	})
	routes.HandleFunc("/auth/items/post/ok", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer abc" {
			respond.WithError(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Header.Get("Content-Type") != "application/json" {
			respond.WithError(w, "Expected application/json content type", http.StatusBadRequest)
			return
		}
		var m map[string]any
		if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
			respond.WithError(w, err.Error(), http.StatusBadRequest)
			return
		}
		respond.WithJSON(w, m, http.StatusCreated)
	})
	return routes
}

type testClient struct {
	Handler http.Handler
}

func (c testClient) Do(req *http.Request) (*http.Response, error) {
	w := httptest.NewRecorder()
	c.Handler.ServeHTTP(w, req)
	return w.Result(), nil
}

func TestClient(t *testing.T) {
	testClient := testClient{Handler: createTestRoutes()}

	ctx := context.Background()
	opts := []jsonapi.Opt{
		jsonapi.WithClient(testClient),
	}

	t.Run("/items/get/ok", func(t *testing.T) {
		resp, ok, err := jsonapi.Get[itemsGetResponse](ctx, "/items/get/ok", opts...)
		if err != nil {
			t.Fatalf("expected no error, got %q", err)
		}
		if diff := cmp.Diff(expectedItemsGetResponse, resp); diff != "" {
			t.Error(diff)
		}
		if !ok {
			t.Error("expected ok to be true")
		}
	})
	t.Run("/items/get/404", func(t *testing.T) {
		_, ok, err := jsonapi.Get[itemsGetResponse](ctx, "/items/get/404", opts...)
		if err != nil {
			t.Fatalf("expected no error, got %q", err)
		}
		if ok {
			t.Error("expected ok to be false")
		}
	})
	t.Run("/items/get/500", func(t *testing.T) {
		_, ok, err := jsonapi.Get[itemsGetResponse](ctx, "/items/get/500", opts...)
		if err == nil {
			t.Fatal("expected an error, got nil")
		}
		ise, isISE := err.(jsonapi.InvalidStatusError)
		if !isISE {
			t.Fatalf("expected InvalidStatusError, got %T", err)
		}
		if ise.Status != http.StatusInternalServerError {
			t.Errorf("expected status code %d, got %d", http.StatusInternalServerError, ise.Status)
		}
		if ok {
			t.Error("expected ok to be false")
		}
	})
	t.Run("/items/post/ok", func(t *testing.T) {
		m := map[string]any{"key": "value"}
		resp, err := jsonapi.Post[map[string]any, map[string]any](ctx, "/items/post/ok", m, opts...)
		if err != nil {
			t.Fatalf("expected no error, got %q", err)
		}
		if diff := cmp.Diff(m, resp); diff != "" {
			t.Error(diff)
		}
	})
	t.Run("/items/post/404", func(t *testing.T) {
		_, err := jsonapi.Post[map[string]any, map[string]any](ctx, "/items/post/404", nil, opts...)
		if err == nil {
			t.Fatal("expected an error, got nil")
		}
		ise, isISE := err.(jsonapi.InvalidStatusError)
		if !isISE {
			t.Fatalf("expected InvalidStatusError, got %T", err)
		}
		if ise.Status != http.StatusNotFound {
			t.Errorf("expected status code %d, got %d", http.StatusNotFound, ise.Status)
		}
		if ise.Body != `{"message":"Not found","statusCode":404}`+"\n" {
			t.Errorf("unexpected body %q", ise.Body)
		}
	})
	t.Run("/items/post/500", func(t *testing.T) {
		_, err := jsonapi.Post[map[string]any, map[string]any](ctx, "/items/post/500", nil, opts...)
		if err == nil {
			t.Fatal("expected an error, got nil")
		}
		ise, isISE := err.(jsonapi.InvalidStatusError)
		if !isISE {
			t.Fatalf("expected InvalidStatusError, got %T", err)
		}
		if ise.Status != http.StatusInternalServerError {
			t.Errorf("expected status code %d, got %d", http.StatusInternalServerError, ise.Status)
		}
		if ise.Body != `{"message":"Internal server error","statusCode":500}`+"\n" {
			t.Errorf("unexpected body %q", ise.Body)
		}
	})
	t.Run("/auth/items/get/ok", func(t *testing.T) {
		resp, ok, err := jsonapi.Get[itemsGetResponse](ctx, "/auth/items/get/ok", jsonapi.WithClient(testClient), jsonapi.WithAuthorization("Bearer abc"))
		if err != nil {
			t.Fatalf("expected no error, got %q", err)
		}
		if diff := cmp.Diff(expectedItemsGetResponse, resp); diff != "" {
			t.Error(diff)
		}
		if !ok {
			t.Error("expected ok to be true")
		}
	})
	t.Run("/auth/items/post/ok", func(t *testing.T) {
		m := map[string]any{"key": "value"}
		resp, err := jsonapi.Post[map[string]any, map[string]any](ctx, "/auth/items/post/ok", m, jsonapi.WithClient(testClient), jsonapi.WithAuthorization("Bearer abc"))
		if err != nil {
			t.Fatalf("expected no error, got %q", err)
		}
		if diff := cmp.Diff(m, resp); diff != "" {
			t.Error(diff)
		}
	})
}
