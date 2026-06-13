package homestate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"jute-dash/apps/hub/internal/pkg/httphelper"
)

var errInvalidHouseholdSettings = errors.New("invalid household settings")

type SettingsStore interface {
	SetupStatus(ctx context.Context) (SetupStatus, error)
	HouseholdSettings(ctx context.Context) (HouseholdSettings, error)
	SaveHouseholdSettings(ctx context.Context, settings HouseholdSettings) (HouseholdSettings, error)
	Rooms(ctx context.Context) ([]RoomConfig, error)
	SaveRooms(ctx context.Context, rooms []RoomConfig) ([]RoomConfig, error)
	Tiles(ctx context.Context) ([]TileConfig, error)
	SaveTiles(ctx context.Context, tiles []TileConfig) ([]TileConfig, error)
	AdapterConnections(ctx context.Context) ([]AdapterConnection, error)
	AdapterConnection(ctx context.Context, id string) (AdapterConnection, error)
	SaveAdapterConnection(ctx context.Context, connection AdapterConnection) (AdapterConnection, error)
}

type Controller struct {
	settings      SettingsStore
	onUpdate      func(HouseholdSettings)
	onRoomsUpdate func([]RoomConfig)
	onTilesUpdate func([]TileConfig)
}

func NewController(
	settings SettingsStore,
	onUpdate func(HouseholdSettings),
	onRoomsUpdate func([]RoomConfig),
	onTilesUpdate func([]TileConfig),
) *Controller {
	return &Controller{
		settings:      settings,
		onUpdate:      onUpdate,
		onRoomsUpdate: onRoomsUpdate,
		onTilesUpdate: onTilesUpdate,
	}
}

func (c *Controller) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/setup/status", c.handleSetupStatus)
	mux.HandleFunc("/api/v1/settings/household", c.handleHouseholdSettings)
	mux.HandleFunc("/api/v1/settings/rooms", c.handleRoomSettings)
	mux.HandleFunc("/api/v1/settings/tiles", c.handleTileSettings)
	mux.HandleFunc("/api/v1/settings/connections", c.handleConnections)
	mux.HandleFunc("/api/v1/home", c.handleHome)
}

func (c *Controller) handleSetupStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httphelper.WriteMethodNotAllowed(w, http.MethodGet)
		return
	}
	status, err := c.settings.SetupStatus(r.Context())
	if err != nil {
		httphelper.WriteError(w, http.StatusInternalServerError, "setup status is unavailable")
		return
	}
	httphelper.WriteJSON(w, http.StatusOK, status)
}

func (c *Controller) handleHouseholdSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		settings, err := c.settings.HouseholdSettings(r.Context())
		if err != nil {
			httphelper.WriteError(w, http.StatusInternalServerError, "household settings are unavailable")
			return
		}
		httphelper.WriteJSON(w, http.StatusOK, settings)
	case http.MethodPatch:
		var req HouseholdSettings
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httphelper.WriteError(w, http.StatusBadRequest, "invalid JSON request body")
			return
		}

		current, err := c.settings.HouseholdSettings(r.Context())
		if err != nil {
			current = HouseholdSettings{}
		}

		merged := mergeHouseholdSettings(current, req)
		if err := validateHouseholdSettings(merged); err != nil {
			httphelper.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}

		saved, err := c.settings.SaveHouseholdSettings(r.Context(), merged)
		if err != nil {
			httphelper.WriteError(w, http.StatusInternalServerError, "household settings could not be saved")
			return
		}

		if c.onUpdate != nil {
			c.onUpdate(saved)
		}

		httphelper.WriteJSON(w, http.StatusOK, saved)
	default:
		httphelper.WriteMethodNotAllowed(w, http.MethodGet+", "+http.MethodPatch)
	}
}

func (c *Controller) handleConnections(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		connections, err := c.settings.AdapterConnections(r.Context())
		if err != nil {
			httphelper.WriteError(w, http.StatusInternalServerError, "adapter connections are unavailable")
			return
		}
		httphelper.WriteJSON(w, http.StatusOK, map[string]any{"connections": connections})
	case http.MethodPut, http.MethodPost:
		var connection AdapterConnection
		if err := json.NewDecoder(r.Body).Decode(&connection); err != nil {
			httphelper.WriteError(w, http.StatusBadRequest, "invalid JSON request body")
			return
		}
		saved, err := c.settings.SaveAdapterConnection(r.Context(), connection)
		if err != nil {
			httphelper.WriteError(w, http.StatusBadRequest, "adapter connection could not be saved")
			return
		}
		httphelper.WriteJSON(w, http.StatusOK, saved)
	default:
		httphelper.WriteMethodNotAllowed(w, http.MethodGet+", "+http.MethodPost+", "+http.MethodPut)
	}
}

func (c *Controller) handleRoomSettings(w http.ResponseWriter, r *http.Request) {
	c.handleConfigSliceSettings(w, r, configSliceSettings[RoomConfig]{
		key: "rooms",
		load: func(ctx context.Context) ([]RoomConfig, error) {
			return c.settings.Rooms(ctx)
		},
		save: func(ctx context.Context, rooms []RoomConfig) ([]RoomConfig, error) {
			saved, err := c.settings.SaveRooms(ctx, rooms)
			if err == nil && c.onRoomsUpdate != nil {
				c.onRoomsUpdate(saved)
			}
			return saved, err
		},
		loadError: "room settings are unavailable",
		saveError: "room settings could not be saved",
	})
}

func (c *Controller) handleTileSettings(w http.ResponseWriter, r *http.Request) {
	c.handleConfigSliceSettingsTile(w, r, configSliceSettings[TileConfig]{
		key: "tiles",
		load: func(ctx context.Context) ([]TileConfig, error) {
			return c.settings.Tiles(ctx)
		},
		save: func(ctx context.Context, tiles []TileConfig) ([]TileConfig, error) {
			saved, err := c.settings.SaveTiles(ctx, tiles)
			if err == nil && c.onTilesUpdate != nil {
				c.onTilesUpdate(saved)
			}
			return saved, err
		},
		loadError: "tile settings are unavailable",
		saveError: "tile settings could not be saved",
	})
}

func (c *Controller) handleHome(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httphelper.WriteMethodNotAllowed(w, http.MethodGet)
		return
	}
	settings, err := c.settings.HouseholdSettings(r.Context())
	if err != nil {
		httphelper.WriteError(w, http.StatusInternalServerError, "home state is unavailable")
		return
	}
	rooms, err := c.settings.Rooms(r.Context())
	if err != nil {
		httphelper.WriteError(w, http.StatusInternalServerError, "home state is unavailable")
		return
	}
	tiles, err := c.settings.Tiles(r.Context())
	if err != nil {
		httphelper.WriteError(w, http.StatusInternalServerError, "home state is unavailable")
		return
	}
	httphelper.WriteJSON(w, http.StatusOK, FromConfig(settings.Home, rooms, tiles, time.Now()))
}

// Helpers

type configSliceSettings[T any] struct {
	key       string
	load      func(context.Context) ([]T, error)
	save      func(context.Context, []T) ([]T, error)
	loadError string
	saveError string
}

func handleConfigSliceSettingsGeneric[T any](
	_ *Controller,
	w http.ResponseWriter,
	r *http.Request,
	settings configSliceSettings[T],
) {
	switch r.Method {
	case http.MethodGet:
		values, err := settings.load(r.Context())
		if err != nil {
			httphelper.WriteError(w, http.StatusInternalServerError, settings.loadError)
			return
		}
		httphelper.WriteJSON(w, http.StatusOK, map[string]any{settings.key: values})
	case http.MethodPut:
		var req map[string][]T
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httphelper.WriteError(w, http.StatusBadRequest, "invalid JSON request body")
			return
		}
		values, err := settings.save(r.Context(), req[settings.key])
		if err != nil {
			if errors.Is(err, ErrInvalidSettings) {
				httphelper.WriteError(w, http.StatusBadRequest, err.Error())
				return
			}
			httphelper.WriteError(w, http.StatusInternalServerError, settings.saveError)
			return
		}
		httphelper.WriteJSON(w, http.StatusOK, map[string]any{settings.key: values})
	default:
		httphelper.WriteMethodNotAllowed(w, http.MethodGet+", "+http.MethodPut)
	}
}

func (c *Controller) handleConfigSliceSettings(
	w http.ResponseWriter,
	r *http.Request,
	settings configSliceSettings[RoomConfig],
) {
	handleConfigSliceSettingsGeneric(c, w, r, settings)
}

func (c *Controller) handleConfigSliceSettingsTile(
	w http.ResponseWriter,
	r *http.Request,
	settings configSliceSettings[TileConfig],
) {
	handleConfigSliceSettingsGeneric(c, w, r, settings)
}

func mergeHouseholdSettings(current, next HouseholdSettings) HouseholdSettings {
	if strings.TrimSpace(next.Home.Name) == "" {
		next.Home.Name = current.Home.Name
	}

	// We handle Display as a generic map or shape.
	currentDisplayBytes, err := json.Marshal(current.Display)
	var currentDisplay DisplaySettings
	if err == nil {
		_ = json.Unmarshal(currentDisplayBytes, &currentDisplay)
	}
	nextDisplayBytes, err := json.Marshal(next.Display)
	var nextDisplay DisplaySettings
	if err == nil {
		_ = json.Unmarshal(nextDisplayBytes, &nextDisplay)
	}

	if strings.TrimSpace(nextDisplay.Theme) == "" {
		nextDisplay.Theme = currentDisplay.Theme
	}
	if strings.TrimSpace(nextDisplay.ColorMode) == "" {
		nextDisplay.ColorMode = currentDisplay.ColorMode
	}
	if strings.TrimSpace(nextDisplay.ThemeID) == "" {
		nextDisplay.ThemeID = currentDisplay.ThemeID
	}
	if strings.TrimSpace(nextDisplay.Density) == "" {
		nextDisplay.Density = currentDisplay.Density
	}
	if strings.TrimSpace(nextDisplay.Motion) == "" {
		nextDisplay.Motion = currentDisplay.Motion
	}
	if nextDisplay.Background == nil {
		nextDisplay.Background = currentDisplay.Background
	}
	if nextDisplay.WidgetChrome == nil {
		nextDisplay.WidgetChrome = currentDisplay.WidgetChrome
	}
	if strings.TrimSpace(nextDisplay.AccentColor) == "" {
		nextDisplay.AccentColor = currentDisplay.AccentColor
	}
	if strings.TrimSpace(nextDisplay.IdleMode) == "" {
		nextDisplay.IdleMode = currentDisplay.IdleMode
	}

	next.Display = nextDisplay

	next.Setup = current.Setup
	return next
}

func validateHouseholdSettings(settings HouseholdSettings) error {
	if strings.TrimSpace(settings.Home.Name) == "" {
		return fmt.Errorf("%w: home.name is required", errInvalidHouseholdSettings)
	}

	probs := ValidateHome(settings.Home)
	if len(probs) > 0 {
		return fmt.Errorf("%w: %s", errInvalidHouseholdSettings, strings.Join(probs, "; "))
	}
	return nil
}
