package registry

import "testing"

func TestRegistryListsCopiesAndFindsByID(t *testing.T) {
	reg := New([]AgentConfig{
		{ID: "one", Name: "One", Enabled: true, Capabilities: []string{"chat"}},
		{ID: "two", Name: "Two", Enabled: false, AuthConfigured: true},
	})

	list := reg.List()
	list[0].Name = "mutated"
	again := reg.List()
	if again[0].Name != "One" {
		t.Fatalf("List() exposed internal slice: %+v", again)
	}

	enabled := reg.Enabled()
	if len(enabled) != 1 || enabled[0].ID != "one" {
		t.Fatalf("Enabled() = %+v", enabled)
	}
	found, ok := reg.Find("two")
	if !ok || found.Name != "Two" || found.AuthAvailable {
		t.Fatalf("Find(two) = %+v, %v", found, ok)
	}
}
