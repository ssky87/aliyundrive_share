package aliyundrive_share

import (
	"time"

	"github.com/alist-org/alist/v3/internal/model"
)

type ErrorResp struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ShareTokenResp struct {
	ShareToken string    `json:"share_token"`
	ExpireTime time.Time `json:"expire_time"`
	ExpiresIn  int       `json:"expires_in"`
}

type ListResp struct {
	Items             []File `json:"items"`
	NextMarker        string `json:"next_marker"`
	PunishedFileCount int    `json:"punished_file_count"`
}

type File struct {
	DriveId      string    `json:"drive_id"`
	DomainId     string    `json:"domain_id"`
	FileId       string    `json:"file_id"`
	ShareId      string    `json:"share_id"`
	Name         string    `json:"name"`
	Type         string    `json:"type"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	ParentFileId string    `json:"parent_file_id"`
	Size         int64     `json:"size"`
	Thumbnail    string    `json:"thumbnail"`
}

func fileToObj(f File) *model.ObjThumb {
	return &model.ObjThumb{
		Object: model.Object{
			ID:       f.FileId,
			Name:     f.Name,
			Size:     f.Size,
			Modified: f.UpdatedAt,
			Ctime:    f.CreatedAt,
			IsFolder: f.Type == "folder",
		},
		Thumbnail: model.Thumbnail{Thumbnail: f.Thumbnail},
	}
}

type ShareLinkResp struct {
	DownloadUrl string `json:"download_url"`
	Url         string `json:"url"`
	Thumbnail   string `json:"thumbnail"`
}

type BatchResult struct {
	Responses []struct {
		Body struct {
			DriveID string `json:"drive_id"`
			FileID  string `json:"file_id"`
		} `json:"body"`
		Status int64 `json:"status"`
	} `json:"responses"`
}

type OpenTokenResp struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	Data         struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	} `json:"data"`
}

type AlipanOpenConfig struct {
        MyDriveId         string
	MyStorePath       string `json:"store_path"`

        OpenAccessToken   string
	OpenRefreshToken  string `json:"refresh_token" required:"true"`
	OpenClientID      string `json:"client_id" required:"false" help:"Keep it empty if you don't have one"`
	OpenClientSecret  string `json:"client_secret" required:"false" help:"Keep it empty if you don't have one"`
	OpenOauthTokenURL string `json:"oauth_token_url" default:"https://api.nn.ci/alist/ali_open/token"`
}
