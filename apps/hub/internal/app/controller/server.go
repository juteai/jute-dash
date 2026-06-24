package controller

import (
	"net/http"

	v1 "jute-dash/apps/hub/api/hub/v1"

	"github.com/labstack/echo/v4"
)

type Server struct {
	handler http.Handler
}

func New(handler http.Handler) *Server {
	return &Server{handler: handler}
}

func (s *Server) serve(ctx echo.Context) error {
	s.handler.ServeHTTP(ctx.Response(), ctx.Request())
	return nil
}

func (s *Server) GetAgents(ctx echo.Context) error                { return s.serve(ctx) }
func (s *Server) PostAgentConversation(ctx echo.Context) error    { return s.serve(ctx) }
func (s *Server) GetBackgrounds(ctx echo.Context) error           { return s.serve(ctx) }
func (s *Server) PostBackground(ctx echo.Context) error           { return s.serve(ctx) }
func (s *Server) GetConfig(ctx echo.Context) error                { return s.serve(ctx) }
func (s *Server) GetEvents(ctx echo.Context) error                { return s.serve(ctx) }
func (s *Server) GetHome(ctx echo.Context) error                  { return s.serve(ctx) }
func (s *Server) GetConnectionKinds(ctx echo.Context) error       { return s.serve(ctx) }
func (s *Server) GetConnections(ctx echo.Context) error           { return s.serve(ctx) }
func (s *Server) PostConnection(ctx echo.Context) error           { return s.serve(ctx) }
func (s *Server) GetHouseholdSettings(ctx echo.Context) error     { return s.serve(ctx) }
func (s *Server) PatchHouseholdSettings(ctx echo.Context) error   { return s.serve(ctx) }
func (s *Server) GetRooms(ctx echo.Context) error                 { return s.serve(ctx) }
func (s *Server) PutRooms(ctx echo.Context) error                 { return s.serve(ctx) }
func (s *Server) GetTiles(ctx echo.Context) error                 { return s.serve(ctx) }
func (s *Server) PutTiles(ctx echo.Context) error                 { return s.serve(ctx) }
func (s *Server) GetSetupStatus(ctx echo.Context) error           { return s.serve(ctx) }
func (s *Server) GetStatus(ctx echo.Context) error                { return s.serve(ctx) }
func (s *Server) GetTTSAudio(ctx echo.Context, _ string) error    { return s.serve(ctx) }
func (s *Server) PostTTSSpeak(ctx echo.Context) error             { return s.serve(ctx) }
func (s *Server) PostTTSStop(ctx echo.Context) error              { return s.serve(ctx) }
func (s *Server) PostVoiceCancel(ctx echo.Context) error          { return s.serve(ctx) }
func (s *Server) PostVoiceMute(ctx echo.Context) error            { return s.serve(ctx) }
func (s *Server) GetVoiceProviders(ctx echo.Context) error        { return s.serve(ctx) }
func (s *Server) PatchVoiceSettings(ctx echo.Context) error       { return s.serve(ctx) }
func (s *Server) GetVoiceStatus(ctx echo.Context) error           { return s.serve(ctx) }
func (s *Server) PostVoiceFinalTranscript(ctx echo.Context) error { return s.serve(ctx) }
func (s *Server) PostVoiceUnmute(ctx echo.Context) error          { return s.serve(ctx) }
func (s *Server) PostVoiceAudio(ctx echo.Context, _ v1.PostVoiceAudioParams) error {
	return s.serve(ctx)
}
func (s *Server) GetWidgetCatalog(ctx echo.Context) error              { return s.serve(ctx) }
func (s *Server) PutWidgetLayout(ctx echo.Context) error               { return s.serve(ctx) }
func (s *Server) PatchWidgetLayoutActiveScreen(ctx echo.Context) error { return s.serve(ctx) }
func (s *Server) GetHealth(ctx echo.Context) error                     { return s.serve(ctx) }
func (s *Server) GetTTSVoices(ctx echo.Context, _ v1.GetTTSVoicesParams) error {
	return s.serve(ctx)
}
func (s *Server) GetWidgetLayout(ctx echo.Context, _ v1.GetWidgetLayoutParams) error {
	return s.serve(ctx)
}
func (s *Server) PostWidgetLayoutReset(ctx echo.Context, _ v1.PostWidgetLayoutResetParams) error {
	return s.serve(ctx)
}

var _ v1.ServerInterface = (*Server)(nil)
