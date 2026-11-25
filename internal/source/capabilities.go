package source

// ListRole groups platform-native lists into a small set of cross-platform roles
// for aggregated multi-account views.
type ListRole string

const (
	RoleLike     ListRole = "like"
	RoleFavorite ListRole = "favorite"
	RoleQueue    ListRole = "queue"
	RoleOther    ListRole = "other"
)

// ListSpec describes one personal list provided by a source.
type ListSpec struct {
	Kind       string   `json:"kind"`
	Role       ListRole `json:"role"`
	HasFolders bool     `json:"hasFolders"`
}

// Descriptor declares the personal lists and user actions supported by a source.
type Descriptor struct {
	ID             string     `json:"id"`
	Lists          []ListSpec `json:"lists"`
	Actions        []string   `json:"actions"`
	VideoActions   []string   `json:"videoActions"`
	GalleryActions []string   `json:"galleryActions"`
	LikeIcon       string     `json:"likeIcon"`
	FolderPicker   bool       `json:"folderPicker"`
}

var builtinDescriptors = []Descriptor{
	{
		ID: "douyin",
		Lists: []ListSpec{
			{Kind: "like", Role: RoleLike},
			{Kind: "favorite", Role: RoleFavorite},
			{Kind: "music", Role: RoleOther},
		},
		Actions:        []string{"like", "favorite"},
		VideoActions:   []string{"like", "favorite"},
		GalleryActions: []string{"like", "favorite"},
		LikeIcon:       "heart",
		FolderPicker:   false,
	},
	{
		ID: "bilibili",
		Lists: []ListSpec{
			{Kind: "favorite", Role: RoleFavorite, HasFolders: true},
			{Kind: "watch_later", Role: RoleQueue},
		},
		Actions:        []string{"favorite", "watch_later"},
		VideoActions:   []string{"favorite", "watch_later"},
		GalleryActions: []string{"favorite"},
		LikeIcon:       "thumbs_up",
		FolderPicker:   true,
	},
}

// Descriptors returns a copy of all registered source descriptors in display order.
func Descriptors() []Descriptor {
	out := make([]Descriptor, len(builtinDescriptors))
	for i, d := range builtinDescriptors {
		out[i] = cloneDescriptor(d)
	}
	return out
}

// DescriptorByID looks up one registered source descriptor by its ID.
func DescriptorByID(id string) (Descriptor, bool) {
	for _, d := range builtinDescriptors {
		if d.ID == id {
			return cloneDescriptor(d), true
		}
	}
	return Descriptor{}, false
}

func cloneDescriptor(d Descriptor) Descriptor {
	d.Lists = append([]ListSpec(nil), d.Lists...)
	d.Actions = append([]string(nil), d.Actions...)
	d.VideoActions = append([]string(nil), d.VideoActions...)
	d.GalleryActions = append([]string(nil), d.GalleryActions...)
	return d
}
