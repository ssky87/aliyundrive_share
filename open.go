package aliyundrive_share

import (
	"fmt"
	"net/http"
	"encoding/json"
	"os"
	"time"

	"github.com/alist-org/alist/v3/drivers/base"
	"github.com/alist-org/alist/v3/pkg/utils"
	"github.com/go-resty/resty/v2"
	log "github.com/sirupsen/logrus"
)

func (d *AliyundriveShare) loadAlipanConfig(filename string) (*AlipanOpenConfig, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var config AlipanOpenConfig
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

func (d *AliyundriveShare) openRequest(uri, method string, callback base.ReqCallback) ([]byte, error) {
	if d.OpenConfig.OpenAccessToken == "" {
		err := d.openRefreshToken()
		if err != nil {
			return nil, err
		}
	}
	var e ErrorResp
	req := base.RestyClient.R().SetError(&e).
		SetHeader("Content-Type", "application/json").
		SetHeader("Authorization", "Bearer "+d.OpenConfig.OpenAccessToken)
	if callback != nil {
		callback(req)
	}
	response, err := req.Execute(method, "https://openapi.alipan.com"+uri)
	if err != nil {
		log.Errorf("%s response: %s", uri, response.String())
		return nil, err
	}
	if response.StatusCode() != 200 {
		if response.StatusCode() == 401 || response.StatusCode() == 429 {
			if response.StatusCode() == 429 {
				time.Sleep(500 * time.Millisecond)
			} else {
				err = d.openRefreshToken()
				if err != nil {
					return nil, err
				}
			}
			return d.openRequest(uri, method, callback)
		} else {
			return nil, fmt.Errorf("%s:%s", e.Code, e.Message)
		}
	}
	return response.Body(), nil
}

func (d *AliyundriveShare) openRefreshToken() error {
	url := "https://openapi.alipan.com/oauth/access_token"
	if d.OpenConfig.OpenOauthTokenURL != "" && d.OpenConfig.OpenClientID == "" {
		url = d.OpenConfig.OpenOauthTokenURL
	}

	var e ErrorResp
	var resp OpenTokenResp
	response, err := base.RestyClient.R().
		//		ForceContentType("application/json").
		SetBody(base.Json{
			"client_id":     d.OpenConfig.OpenClientID,
			"client_secret": d.OpenConfig.OpenClientSecret,
			"grant_type":    "refresh_token",
			"refresh_token": d.OpenConfig.OpenRefreshToken,
		}).SetResult(&resp).SetError(&e).
		Post(url)
	if err != nil {
		log.Errorf("openRefreshToken response: %s", response.String())
		return err
	}
	if 200 != response.StatusCode() {
		return fmt.Errorf("failed to get openRefreshToken: %s:%s", e.Code, e.Message)
	}
	if resp.RefreshToken == "" {
		d.OpenConfig.OpenRefreshToken, d.OpenConfig.OpenAccessToken = resp.Data.RefreshToken, resp.Data.AccessToken
	} else {
		d.OpenConfig.OpenRefreshToken, d.OpenConfig.OpenAccessToken = resp.RefreshToken, resp.AccessToken
	}
	return nil
}

func (d *AliyundriveShare) getFileUrlFromOpen(fileId string) (string, error) {
	resp, err := d.openRequest("/adrive/v1.0/openFile/getDownloadUrl", http.MethodPost, func(req *resty.Request) {
		req.SetBody(base.Json{
			"drive_id":   d.OpenConfig.MyDriveId,
			"file_id":    fileId,
			"expire_sec": 14400,
		})
	})
	data := `{"requests":[{"body":{"drive_id":"` + d.OpenConfig.MyDriveId + `","file_id":"` + fileId + `"},"headers":{"Content-Type":"application/json"},"id":"0","method":"POST","url":"/file/delete"}],"resource":"file"}`
	_, _ = d.request("https://api.alipan.com/adrive/v4/batch", http.MethodPost, func(req *resty.Request) {
		req.SetBody(data)
	})
	if err != nil {
		return "", err
	}
	return utils.Json.Get(resp, "url").ToString(), nil
}
