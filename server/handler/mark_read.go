package handler

import (
	"github.com/ArtalkJS/ArtalkGo/internal/query"
	"github.com/ArtalkJS/ArtalkGo/server/common"
	"github.com/gofiber/fiber/v2"
)

type ParamsMarkRead struct {
	NotifyKey string `form:"notify_key"`

	Name    string `form:"name"`
	Email   string `form:"email"`
	AllRead bool   `form:"all_read"`

	SiteName string
	SiteID   uint
	SiteAll  bool
}

// POST /api/mark-read
func MarkRead(router fiber.Router) {
	router.Post("/mark-read", func(c *fiber.Ctx) error {
		var p ParamsMarkRead
		if isOK, resp := common.ParamsDecode(c, &p); !isOK {
			return resp
		}

		// use site
		common.UseSite(c, &p.SiteName, &p.SiteID, &p.SiteAll)

		// all read
		if p.AllRead {
			if p.Name == "" || p.Email == "" {
				return common.RespError(c, "need username and email")
			}

			user := query.FindUser(p.Name, p.Email)
			err := query.UserNotifyMarkAllAsRead(user.ID)
			if err != nil {
				return common.RespError(c, err.Error())
			}

			return common.RespSuccess(c)
		}

		// find notify
		notify := query.FindNotifyByKey(p.NotifyKey)
		if notify.IsEmpty() {
			return common.RespError(c, "notify key is wrong")
		}

		if notify.IsRead {
			return common.RespSuccess(c)
		}

		// update notify
		err := query.NotifySetRead(&notify)
		if err != nil {
			return common.RespError(c, "notify save error")
		}

		return common.RespSuccess(c)
	})
}
