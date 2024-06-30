package aliyundrive_share

import (
	"context"
	"fmt"
	"net/http"
	"time"
	"os"
	"path/filepath"

	"github.com/Xhofe/rateg"
	"github.com/alist-org/alist/v3/cmd/flags"
	"github.com/alist-org/alist/v3/drivers/base"
	"github.com/alist-org/alist/v3/internal/driver"
	"github.com/alist-org/alist/v3/internal/errs"
	"github.com/alist-org/alist/v3/internal/model"
	"github.com/alist-org/alist/v3/pkg/cron"
	"github.com/alist-org/alist/v3/pkg/utils"
	"github.com/go-resty/resty/v2"
	log "github.com/sirupsen/logrus"
)

type AliyundriveShare struct {
	model.Storage
	Addition
	AccessToken string
	ShareToken  string
	DriveId     string
	cron        *cron.Cron

	OpenConfig *AlipanOpenConfig

	limitList func(ctx context.Context, dir model.Obj) ([]model.Obj, error)
	limitLink func(ctx context.Context, file model.Obj) (*model.Link, error)
}

func (d *AliyundriveShare) Config() driver.Config {
	return config
}

func (d *AliyundriveShare) GetAddition() driver.Additional {
	return &d.Addition
}

func (d *AliyundriveShare) Init(ctx context.Context) error {
	if flags.ForceBinDir {
		if !filepath.IsAbs(flags.DataDir) {
			ex, err := os.Executable()
			if err != nil {
				utils.Log.Fatal(err)
			}
			exPath := filepath.Dir(ex)
			flags.DataDir = filepath.Join(exPath, flags.DataDir)
		}
	}
	configPath := filepath.Join(flags.DataDir, "alipan.json")
	conf ,err := d.loadAlipanConfig(configPath)
	if err != nil {
		log.Errorf("Failed to load Alipan config: %v", err)
	}
	d.OpenConfig = conf
	log.Errorf(d.OpenConfig.MyStorePath)

	res, err := d.request("https://user.alipan.com/v2/user/get", http.MethodPost, nil)
	if err != nil {
		return err
	}
	d.OpenConfig.MyDriveId = utils.Json.Get(res, "resource_drive_id").ToString()

	err = d.refreshToken()
	if err != nil {
		return err
	}
	err = d.getShareToken()
	if err != nil {
		return err
	}
	d.cron = cron.NewCron(time.Hour * 2)
	d.cron.Do(func() {
		err := d.refreshToken()
		if err != nil {
			log.Errorf("%+v", err)
		}
	})
	d.limitList = rateg.LimitFnCtx(d.list, rateg.LimitFnOption{
		Limit:  4,
		Bucket: 1,
	})
	d.limitLink = rateg.LimitFnCtx(d.link, rateg.LimitFnOption{
		Limit:  1,
		Bucket: 1,
	})
	return nil
}

func (d *AliyundriveShare) Drop(ctx context.Context) error {
	if d.cron != nil {
		d.cron.Stop()
	}
	d.DriveId = ""
	return nil
}

func (d *AliyundriveShare) List(ctx context.Context, dir model.Obj, args model.ListArgs) ([]model.Obj, error) {
	if d.limitList == nil {
		return nil, fmt.Errorf("driver not init")
	}
	return d.limitList(ctx, dir)
}

func (d *AliyundriveShare) list(ctx context.Context, dir model.Obj) ([]model.Obj, error) {
	files, err := d.getFiles(dir.GetID())
	if err != nil {
		return nil, err
	}
	return utils.SliceConvert(files, func(src File) (model.Obj, error) {
		return fileToObj(src), nil
	})
}

func (d *AliyundriveShare) Link(ctx context.Context, file model.Obj, args model.LinkArgs) (*model.Link, error) {
	if d.limitLink == nil {
		return nil, fmt.Errorf("driver not init")
	}
	return d.limitLink(ctx, file)
}

func (d *AliyundriveShare) link(ctx context.Context, file model.Obj) (*model.Link, error) {
	log.Debugf("[%s] File size is: %d",d.ShareId, file.GetSize())

	if d.OpenConfig != nil && file.GetSize()/1024 > 10*1024 {
		log.Debugf("[%s] 转存到自己自己网盘",d.ShareId)
		tmpFileId, err := d.copyFile(d.ShareId, file.GetID())
		if err != nil {
			return nil, err
		}
		if tmpFileId == "" {
			return nil, fmt.Errorf("save file failed")
		}

		url, err := d.getFileUrlFromOpen(tmpFileId)
		if err != nil {
			return nil, err
		}
		if url == "" {
			return nil, fmt.Errorf("get file url failed")
		}
		log.Infof("[%s] 获得直链:%s",d.ShareId,url)
		exp := 30 * time.Minute
		return &model.Link{
			URL:        url,
			Expiration: &exp,
		}, nil

	} else {

		data := base.Json{
			"drive_id": d.DriveId,
			"file_id":  file.GetID(),
			// // Only ten minutes lifetime
			"expire_sec": 600,
			"share_id":   d.ShareId,
		}
		var resp ShareLinkResp
		_, err := d.request("https://api.alipan.com/v2/file/get_share_link_download_url", http.MethodPost, func(req *resty.Request) {
			req.SetHeader(CanaryHeaderKey, CanaryHeaderValue).SetBody(data).SetResult(&resp)
		})
		if err != nil {
			return nil, err
		}
		return &model.Link{
			Header: http.Header{
				"Referer": []string{"https://www.alipan.com/"},
			},
			URL: resp.DownloadUrl,
		}, nil
	}
}

func (d *AliyundriveShare) Other(ctx context.Context, args model.OtherArgs) (interface{}, error) {
	var resp base.Json
	var url string
	data := base.Json{
		"share_id": d.ShareId,
		"file_id":  args.Obj.GetID(),
	}
	switch args.Method {
	case "doc_preview":
		url = "https://api.alipan.com/v2/file/get_office_preview_url"
	case "video_preview":
		url = "https://api.alipan.com/v2/file/get_video_preview_play_info"
		data["category"] = "live_transcoding"
	default:
		return nil, errs.NotSupport
	}
	_, err := d.request(url, http.MethodPost, func(req *resty.Request) {
		req.SetBody(data).SetResult(&resp)
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
}

var _ driver.Driver = (*AliyundriveShare)(nil)
