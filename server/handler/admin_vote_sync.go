package handler

import (
	"github.com/ArtalkJS/ArtalkGo/internal/query"
	"github.com/ArtalkJS/ArtalkGo/server/common"
	"github.com/gofiber/fiber/v2"
)

type ParamsAdminVoteSync struct {
}

// POST /api/admin/vote-sync
func AdminVoteSync(router fiber.Router) {
	router.Post("/vote-sync", func(c *fiber.Ctx) error {
		var p ParamsAdminVoteSync
		if isOK, resp := common.ParamsDecode(c, &p); !isOK {
			return resp
		}

		if !common.GetIsSuperAdmin(c) {
			return common.RespError(c, "无权访问")
		}

		query.VoteSync()

		return common.RespSuccess(c)
	})
}
