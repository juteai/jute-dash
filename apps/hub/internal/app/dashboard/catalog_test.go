package dashboard

import (
	"testing"

	_ "jute-dash/widgets/spotify/hub"
)

func TestRegisteredCatalogUsesWidgetCatalogShape(t *testing.T) {
	catalog := RegisteredCatalog()
	var spotify *WidgetCatalogItem
	for i := range catalog {
		if catalog[i].Kind == "spotify" {
			spotify = &catalog[i]
			break
		}
	}
	if spotify == nil {
		t.Fatal("spotify widget was not registered")
	}
	if len(spotify.ConnectionRequirements) != 1 {
		t.Fatalf("expected spotify connection requirement, got %#v", spotify.ConnectionRequirements)
	}
	requirement := spotify.ConnectionRequirements[0]
	if requirement.Kind != "spotify" {
		t.Fatalf("expected spotify connection kind, got %q", requirement.Kind)
	}
	if len(requirement.Fields) < 2 {
		t.Fatal("expected typed connection setup fields")
	}
	if requirement.Fields[0].ID != "auth_type" || requirement.Fields[1].ID != "client_id" {
		t.Fatalf("expected typed fields from root widget catalog shape, got %#v", requirement.Fields)
	}
}
