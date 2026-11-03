package app

import "errors"

type LibraryFolder struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Count int    `json:"count"`
}

func (l *Library) Folders(accountID string) ([]LibraryFolder, error) {
	if l.app.Store == nil {
		return nil, errors.New("STORE_FAILED")
	}
	ctx, cancel := accountContext()
	defer cancel()
	folders, err := l.app.Store.Folders(ctx, accountID)
	if err != nil {
		return nil, errors.New("STORE_FAILED")
	}
	out := make([]LibraryFolder, len(folders))
	for i, f := range folders {
		out[i] = LibraryFolder{ID: f.ID, Title: f.Title, Count: f.Count}
	}
	return out, nil
}
