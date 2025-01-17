package handler

import (
	"strings"

	"github.com/ArtalkJS/ArtalkGo/internal/cache"
	"github.com/ArtalkJS/ArtalkGo/internal/db"
	"github.com/ArtalkJS/ArtalkGo/internal/entity"
	"github.com/ArtalkJS/ArtalkGo/internal/query"
	"github.com/ArtalkJS/ArtalkGo/internal/utils"
	"github.com/ArtalkJS/ArtalkGo/server/common"
	"github.com/gofiber/fiber/v2"
)

type ParamsAdminSiteEdit struct {
	// 查询值
	ID uint `form:"id" validate:"required"`

	// 修改值
	Name string `form:"name"`
	Urls string `form:"urls"`
}

// POST /api/admin/site-edit
func AdminSiteEdit(router fiber.Router) {
	router.Post("/site-edit", func(c *fiber.Ctx) error {
		var p ParamsAdminSiteEdit
		if isOK, resp := common.ParamsDecode(c, &p); !isOK {
			return resp
		}

		site := query.FindSiteByID(p.ID)
		if site.IsEmpty() {
			return common.RespError(c, "site 不存在")
		}

		// 站点操作权限检查
		if !common.IsAdminHasSiteAccess(c, site.Name) {
			return common.RespError(c, "无权操作")
		}

		if strings.TrimSpace(p.Name) == "" {
			return common.RespError(c, "site 名称不能为空白字符")
		}

		// 重命名合法性检测
		modifyName := p.Name != site.Name
		if modifyName && !query.FindSite(p.Name).IsEmpty() {
			return common.RespError(c, "site 已存在，请换个名称")
		}

		// urls 合法性检测
		if p.Urls != "" {
			for _, url := range utils.SplitAndTrimSpace(p.Urls, ",") {
				if !utils.ValidateURL(url) {
					return common.RespError(c, "Invalid url exist")
				}
			}
		}

		// 预先删除缓存，防止修改主键原有 site_name 占用问题
		cache.SiteCacheDel(&site)

		// 同步变更 site_name
		if modifyName {
			var comments []entity.Comment
			var pages []entity.Page

			db.DB().Where("site_name = ?", site.Name).Find(&comments)
			db.DB().Where("site_name = ?", site.Name).Find(&pages)

			for _, comment := range comments {
				comment.SiteName = p.Name
				query.UpdateComment(&comment)
			}
			for _, page := range pages {
				page.SiteName = p.Name
				query.UpdatePage(&page)
			}
		}

		// 修改 site
		site.Name = p.Name
		site.Urls = p.Urls

		err := query.UpdateSite(&site)
		if err != nil {
			return common.RespError(c, "site 保存失败")
		}

		// 刷新 CORS 可信域名
		common.ReloadCorsAllowOrigins()

		return common.RespData(c, common.Map{
			"site": query.CookSite(&site),
		})
	})
}
