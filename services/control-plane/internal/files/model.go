package files

import "time"

type FileEntry struct {
	Name        string    `json:"name"`
	Path        string    `json:"path"`
	IsDir       bool      `json:"is_dir"`
	IsSymlink   bool      `json:"is_symlink"`
	Size        int64     `json:"size"`
	Permissions string    `json:"permissions"`
	ModTime     time.Time `json:"mod_time"`
	MimeType    string    `json:"mime_type"`
	IsBinary    bool      `json:"is_binary"`
}

type DirectoryListing struct {
	CurrentPath string      `json:"current_path"`
	ParentPath  string      `json:"parent_path,omitempty"`
	Entries     []FileEntry `json:"entries"`
	TotalItems  int         `json:"total_items"`
}

type FileContent struct {
	Path        string    `json:"path"`
	Content     string    `json:"content"`
	Size        int64     `json:"size"`
	Permissions string    `json:"permissions"`
	ModTime     time.Time `json:"mod_time"`
	IsBinary    bool      `json:"is_binary"`
}
