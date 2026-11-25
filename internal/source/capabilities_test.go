package source

import "testing"

func TestDescriptors(t *testing.T) {
	all := Descriptors()
	if len(all) < 2 {
		t.Fatalf("expected built-in descriptors, got %d", len(all))
	}
	seen := map[string]bool{}
	for _, d := range all {
		if d.ID == "" || len(d.Lists) == 0 {
			t.Fatalf("invalid descriptor: %+v", d)
		}
		if seen[d.ID] {
			t.Fatalf("duplicate descriptor id %q", d.ID)
		}
		seen[d.ID] = true
		for _, list := range d.Lists {
			switch list.Role {
			case RoleLike, RoleFavorite, RoleQueue, RoleOther:
			default:
				t.Fatalf("source %q list %q has invalid role %q", d.ID, list.Kind, list.Role)
			}
		}
	}
	// Mutating returned copies must not affect the registry.
	all[0].Lists[0].Kind = "mutated"
	dy, ok := DescriptorByID("douyin")
	if !ok || len(dy.Lists) != 4 || dy.Lists[0].Kind != "like" || dy.Lists[1].Kind != "favorite" || dy.Lists[2].Kind != "recommend" || dy.Lists[2].Role != RoleOther || dy.Lists[3].Kind != "music" {
		t.Fatalf("unexpected douyin descriptor: %+v ok=%v", dy, ok)
	}
	bl, ok := DescriptorByID("bilibili")
	if !ok || !bl.FolderPicker || !bl.Lists[0].HasFolders {
		t.Fatalf("unexpected bilibili descriptor: %+v ok=%v", bl, ok)
	}
	if _, ok := DescriptorByID("unknown"); ok {
		t.Fatal("unexpected descriptor for unknown source")
	}
}
