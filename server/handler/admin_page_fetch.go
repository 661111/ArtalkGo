package handler

import (
	"fmt"

	"github.com/ArtalkJS/ArtalkGo/internal/config"
	"github.com/ArtalkJS/ArtalkGo/internal/db"
	"github.com/ArtalkJS/ArtalkGo/internal/entity"
	"github.com/ArtalkJS/ArtalkGo/internal/query"
	"github.com/ArtalkJS/ArtalkGo/server/common"
	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"
)

type ParamsAdminPageFetch struct {
	ID       uint `form:"id"`
	SiteName string

	GetStatus bool `form:"get_status"`
}

var allPageFetching = false
var allPageFetchDone = 0
var allPageFetchTotal = 0

// POST /api/admin/page-fetch
func AdminPageFetch(router fiber.Router) {
	router.Post("/page-fetch", func(c *fiber.Ctx) error {
		var p ParamsAdminPageFetch
		if isOK, resp := common.ParamsDecode(c, &p); !isOK {
			return resp
		}

		common.UseSite(c, &p.SiteName, nil, nil)

		// 状态获取
		if p.GetStatus {
			if allPageFetching {
				return common.RespData(c, common.Map{
					"msg":         fmt.Sprintf("已完成 %d 共 %d 个", allPageFetchDone, allPageFetchTotal),
					"is_progress": true,
				})
			} else {
				return common.RespData(c, common.Map{
					"msg":         "",
					"is_progress": false,
				})
			}
		}

		// 更新全部站点
		if p.SiteName != "" {
			if allPageFetching {
				return common.RespError(c, "任务正在进行中，请稍等片刻")
			}

			// 异步执行
			go func() {
				allPageFetching = true
				allPageFetchDone = 0
				allPageFetchTotal = 0
				var pages []entity.Page
				db := db.DB().Model(&entity.Page{})
				if p.SiteName != config.ATK_SITE_ALL {
					db = db.Where(&entity.Page{SiteName: p.SiteName})
				}
				db.Find(&pages)

				allPageFetchTotal = len(pages)
				for _, p := range pages {
					if err := query.FetchPageFromURL(&p); err != nil {
						logrus.Error(c, "[api_admin_page_fetch] page fetch error: "+err.Error())
					} else {
						allPageFetchDone++
					}
				}
				allPageFetching = false
			}()

			return common.RespSuccess(c)
		}

		page := query.FindPageByID(p.ID)
		if page.IsEmpty() {
			return common.RespError(c, "page not found")
		}

		if !common.IsAdminHasSiteAccess(c, page.SiteName) {
			return common.RespError(c, "无权操作")
		}

		if err := query.FetchPageFromURL(&page); err != nil {
			return common.RespError(c, "page fetch error: "+err.Error())
		}

		return common.RespData(c, common.Map{
			"page": query.CookPage(&page),
		})
	})
}
