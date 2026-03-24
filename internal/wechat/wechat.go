package wechat

import (
	"pkuphysu-backend/internal/config"

	wechat "github.com/silenceper/wechat/v2"
	"github.com/silenceper/wechat/v2/cache"
	"github.com/silenceper/wechat/v2/officialaccount"
	offConfig "github.com/silenceper/wechat/v2/officialaccount/config"
)

var (
	wc              *wechat.Wechat
	memory          *cache.Memory
	cfg             *offConfig.Config
	OfficialAccount *officialaccount.OfficialAccount
)

func Init() {
	wc = wechat.NewWechat()
	memory = cache.NewMemory()
	cfg = &offConfig.Config{
		AppID:     config.Conf.Wechat.AppID,
		AppSecret: config.Conf.Wechat.AppSecret,
		Token:     config.Conf.Wechat.Token,
		Cache:     memory,
	}

	OfficialAccount = wc.GetOfficialAccount(cfg)
}
