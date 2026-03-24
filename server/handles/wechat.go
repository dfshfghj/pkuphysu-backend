package handles

import (
	"fmt"
	"pkuphysu-backend/internal/wechat"

	"github.com/gin-gonic/gin"

	"github.com/silenceper/wechat/v2/officialaccount/message"
)

func Message(c *gin.Context) {
	server := wechat.OfficialAccount.GetServer(c.Request, c.Writer)
	server.SetMessageHandler(func(msg *message.MixMessage) *message.Reply {

		if msg.Content == "openid" {
			text := message.NewText(string(msg.FromUserName))
			return &message.Reply{MsgType: message.MsgTypeText, MsgData: text}
		} else {
			text := message.NewText(msg.Content)
			return &message.Reply{MsgType: message.MsgTypeText, MsgData: text}
		}

	})

	err := server.Serve()
	if err != nil {
		fmt.Println(err)
		return
	}

	server.Send()
}
