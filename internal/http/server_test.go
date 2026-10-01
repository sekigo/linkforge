package http

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sekigo/linkforge/internal/domain"
	"github.com/sekigo/linkforge/internal/service"
	"github.com/sekigo/linkforge/internal/storage"
	"github.com/stretchr/testify/require"
)

type fakeStorage struct {
	seq    int64
	links  map[string]*domain.Link
	byCode map[string]string // code -> url
}

func (f *fakeStorage) NextSequence(_ context.Context) (int64, error) {
	f.seq++
	return f.seq, nil
}

func (f *fakeStorage) InsertLink(_ context.Context, code, rawURL string) (*domain.Link, error) {
	link := &domain.Link{
		ID:        f.seq,
		Code:      code,
		URL:       rawURL,
		CreatedAt: time.Now(),
	}
	f.links[rawURL] = link
	f.byCode[code] = rawURL
	return link, nil
}

func (f *fakeStorage) GetLinkByCode(_ context.Context, code string) (*domain.Link, error) {
	url, ok := f.byCode[code]
	if !ok {
		return nil, storage.ErrNotFound
	}
	return &domain.Link{Code: code, URL: url}, nil
}

type fakeCache struct {
	data map[string]string
}

func (f *fakeCache) GetURL(_ context.Context, code string) (string, error) {
	v, ok := f.data[code]
	if !ok {
		return "", storage.ErrNotFound
	}
	return v, nil
}

func (f *fakeCache) SetURL(_ context.Context, code, url string) error {
	if f.data == nil {
		f.data = make(map[string]string)
	}
	f.data[code] = url
	return nil
}

func buildTestRouter(t *testing.T) (*handlers, *fakeStorage) {
	t.Helper()
	storage := &fakeStorage{links: make(map[string]*domain.Link), byCode: make(map[string]string)}
	cache := &fakeCache{data: make(map[string]string)}
	sh := service.NewShortener(storage, cache, "http://short.url")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return &handlers{sh: sh, logger: logger}, storage
}

func TestHealthz(t *testing.T) {
	h, _ := buildTestRouter(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	h.health(w, r)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "ok", w.Body.String())
}

func TestCreateLink(t *testing.T) {
	h, storage := buildTestRouter(t)

	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantCode   bool
		wantError  bool
	}{
		{
			name:       "valid URL",
			body:       `{"url":"https://example.com"}`,
			wantStatus: http.StatusCreated,
			wantCode:   true,
		},
		{
			name:       "empty body",
			body:       `{}`,
			wantStatus: http.StatusBadRequest,
			wantError:  true,
		},
		{
			name:       "invalid JSON",
			body:       `{`,
			wantStatus: http.StatusBadRequest,
			wantError:  true,
		},
		{
			name:       "URL without scheme",
			body:       `{"url":"example.com"}`,
			wantStatus: http.StatusBadRequest,
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage.links = make(map[string]*domain.Link)
			storage.byCode = make(map[string]string)
			storage.seq = 0

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, "/api/v1/links", strings.NewReader(tt.body))
			r.Header.Set("Content-Type", "application/json")
			h.createLink(w, r)
			require.Equal(t, tt.wantStatus, w.Code)

			if tt.wantCode {
				var resp createLinkResponse
				require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
				require.NotEmpty(t, resp.Code)
				require.NotEmpty(t, resp.ShortURL)
				require.NotEmpty(t, resp.URL)
			}
			if tt.wantError {
				var errResp map[string]string
				require.NoError(t, json.NewDecoder(w.Body).Decode(&errResp))
				require.Contains(t, errResp, "error")
			}
		})
	}
}

func TestRedirect(t *testing.T) {
	h, storage := buildTestRouter(t)

	router := chi.NewRouter()
	router.Get("/{code}", h.redirect)

	storage.seq = 1
	storage.byCode["abc123"] = "https://example.com"

	t.Run("existing code", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/abc123", nil)
		router.ServeHTTP(w, r)
		require.Equal(t, http.StatusFound, w.Code)
		require.Equal(t, "https://example.com", w.Header().Get("Location"))
	})

	t.Run("not found", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/notexist", nil)
		router.ServeHTTP(w, r)
		require.Equal(t, http.StatusNotFound, w.Code)
	})
}