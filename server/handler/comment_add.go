package handler

import (
	"strings"

	"github.com/ArtalkJS/ArtalkGo/internal/anti_spam"
	"github.com/ArtalkJS/ArtalkGo/internal/config"
	"github.com/ArtalkJS/ArtalkGo/internal/entity"
	"github.com/ArtalkJS/ArtalkGo/internal/notify_launcher"
	"github.com/ArtalkJS/ArtalkGo/internal/query"
	"github.com/ArtalkJS/ArtalkGo/internal/utils"
	"github.com/ArtalkJS/ArtalkGo/server/common"
	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"
)

type ParamsAdd struct {
	Name    string `form:"name"`
	Email   string `form:"email"`
	Link    string `form:"link"`
	Content string `form:"content" validate:"required"`
	Rid     uint   `form:"rid"`
	UA      string `form:"ua"`

	PageKey   string `form:"page_key" validate:"required"`
	PageTitle string `form:"page_title"`

	Token    string `form:"token"`
	SiteName string
	SiteID   uint
}

type ResponseAdd struct {
	Comment entity.CookedComment `json:"comment"`
}

// GET /api/add
func CommentAdd(router fiber.Router) {
	router.Post("/add", func(c *fiber.Ctx) error {
		var p ParamsAdd
		if isOK, resp := common.ParamsDecode(c, &p); !isOK {
			return resp
		}

		if strings.TrimSpace(p.Name) == "" {
			return common.RespError(c, "昵称不能为空")
		}
		if strings.TrimSpace(p.Email) == "" {
			return common.RespError(c, "邮箱不能为空")
		}

		if !utils.ValidateEmail(p.Email) {
			return common.RespError(c, "Invalid email")
		}
		if p.Link != "" && !utils.ValidateURL(p.Link) {
			return common.RespError(c, "Invalid link")
		}

		ip := c.IP()
		ua := string(c.Request().Header.UserAgent())

		// 允许传入修正后的 UA
		if p.UA != "" {
			ua = p.UA
		}

		// record action for limiting action
		common.RecordAction(c)

		// use site
		common.UseSite(c, &p.SiteName, &p.SiteID, nil)

		// find page
		page := query.FindCreatePage(p.PageKey, p.PageTitle, p.SiteName)

		// check if the user is allowed to comment
		if isAllowed, resp := common.CheckIsAllowed(c, p.Name, p.Email, page, p.SiteName); !isAllowed {
			return resp
		}

		// check reply comment
		var parentComment entity.Comment
		if p.Rid != 0 {
			parentComment = query.FindComment(p.Rid)
			if parentComment.IsEmpty() {
				return common.RespError(c, "找不到父评论")
			}
			if parentComment.PageKey != p.PageKey {
				return common.RespError(c, "与父评论的 pageKey 不一致")
			}
			if !parentComment.IsAllowReply() {
				return common.RespError(c, "不允许回复该评论")
			}
		}

		// find user
		user := query.FindCreateUser(p.Name, p.Email, p.Link)
		if user.ID == 0 || page.Key == "" {
			logrus.Error("Cannot get user or page")
			return common.RespError(c, "评论失败")
		}

		// update user
		user.Link = p.Link
		user.LastIP = ip
		user.LastUA = ua
		user.Name = p.Name // for 若用户修改用户名大小写
		user.Email = p.Email
		query.UpdateUser(&user)

		comment := entity.Comment{
			Content:  p.Content,
			PageKey:  page.Key,
			SiteName: p.SiteName,

			UserID: user.ID,
			IP:     ip,
			UA:     ua,

			Rid: p.Rid,

			IsPending:   false,
			IsCollapsed: false,
			IsPinned:    false,
		}

		// default comment type
		if !common.CheckIsAdminReq(c) && config.Instance.Moderator.PendingDefault {
			// 不是管理员评论 && 配置开启评论默认待审
			comment.IsPending = true
		}

		// save to database
		err := query.CreateComment(&comment)
		if err != nil {
			logrus.Error("Save Comment error: ", err)
			return common.RespError(c, "评论失败")
		}

		// 异步执行
		go func() {
			// Page Update
			if query.CookPage(&page).URL != "" && page.Title == "" {
				query.FetchPageFromURL(&page)
			}

			// 垃圾检测
			if !common.CheckIsAdminReq(c) { // 忽略检查管理员
				anti_spam.SyncSpamCheck(&comment, c) // 同步执行
			}

			// 通知发送
			notify_launcher.SendNotify(&comment, &parentComment)
		}()

		return common.RespData(c, ResponseAdd{
			Comment: query.CookComment(&comment),
		})
	})
}
