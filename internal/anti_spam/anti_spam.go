package anti_spam

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"strconv"
	"strings"

	"github.com/ArtalkJS/ArtalkGo/internal/config"
	"github.com/ArtalkJS/ArtalkGo/internal/entity"
	"github.com/ArtalkJS/ArtalkGo/internal/query"
	"github.com/ArtalkJS/ArtalkGo/internal/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"
)

var AntiSpamReplaceKeywords *[]string

func SyncSpamCheck(comment *entity.Comment, fiberCtx *fiber.Ctx) {
	// 拦截评论
	BlockCommentBy := func(blocker string) {
		logrus.Info(fmt.Sprintf("[垃圾拦截] %s 成功拦截评论 ID=%d 内容=%s", blocker, comment.ID, strconv.Quote(comment.Content)))
		if comment.IsPending {
			return
		}
		comment.IsPending = true // 改为待审状态
		query.UpdateComment(comment)
	}

	// 拦截失败处理
	BlockFailBy := func(blocker string, err error) {
		logrus.Error(fmt.Sprintf("[垃圾拦截] %s 拦截发生错误 ID=%d 错误信息: %s", blocker, comment.ID, strconv.Quote(comment.Content)), err)
	}

	// 统一拦截处理
	ApiCommonHandle := func(blocker string, isPass bool, err error) {
		// ApiFailBlock mode
		isApiFailBlock := config.Instance.Moderator.ApiFailBlock

		if err != nil {
			// Api 发生错误
			BlockFailBy(blocker, err) // 报告错误
			if isApiFailBlock {
				BlockCommentBy(blocker) // 仍然拦截
			}
		} else if !isPass {
			// Api 未发生错误，并且 not pass
			BlockCommentBy(blocker) // 拦截评论
		}
	}

	// Prepare data for Spam-Check
	user := query.FetchUserForComment(comment)
	siteURL := ""
	if comment.SiteName != "" {
		site := query.FindSite(comment.SiteName)
		siteURL = query.CookSite(&site).FirstUrl
	}
	if siteURL == "" { // 从 referer 中提取网站
		if pr, err := url.Parse(string(fiberCtx.Request().Header.Referer())); err == nil && pr.Scheme != "" && pr.Host != "" {
			siteURL = fmt.Sprintf("%s://%s", pr.Scheme, pr.Host)
		}
	}

	// Akismet
	akismetKey := strings.TrimSpace(config.Instance.Moderator.AkismetKey)
	if akismetKey != "" {
		isPass, err := Akismet(&AkismetParams{
			Blog: siteURL,

			UserIP:    fiberCtx.IP(),
			UserAgent: string(fiberCtx.Request().Header.UserAgent()),

			CommentType:        "comment",
			CommentAuthor:      user.Name,
			CommentAuthorEmail: user.Email,
			CommentContent:     comment.Content,
		}, akismetKey)

		ApiCommonHandle("Akismet", isPass, err)
	}

	// 腾讯云
	tencentConf := config.Instance.Moderator.Tencent
	if tencentConf.Enabled {
		isPass, err := Tencent(TencentParams{
			SecretID:  tencentConf.SecretID,
			SecretKey: tencentConf.SecretKey,
			Region:    tencentConf.Region,

			Content:   comment.Content,
			CommentID: comment.ID,
			UserID:    comment.UserID,
			UserIP:    comment.IP,
			UserName:  user.Name,
		})

		ApiCommonHandle("腾讯云", isPass, err)
	}

	// 阿里云
	aliyunConf := config.Instance.Moderator.Aliyun
	if aliyunConf.Enabled {
		isPass, err := Aliyun(AliyunParams{
			AccessKeyID:     aliyunConf.AccessKeyID,
			AccessKeySecret: aliyunConf.AccessKeySecret,
			Region:          aliyunConf.Region,

			Content:   comment.Content,
			CommentID: comment.ID,
		})

		ApiCommonHandle("阿里云", isPass, err)
	}

	// 关键字过滤
	keywordsConf := config.Instance.Moderator.Keywords
	if keywordsConf.Enabled {
		// 懒加载，初始化
		if AntiSpamReplaceKeywords == nil {
			AntiSpamReplaceKeywords = &[]string{}
			// 加载文件
			for _, f := range keywordsConf.Files {
				buf, err := ioutil.ReadFile(f)
				if err != nil {
					logrus.Error("关键词词库文件 " + f + " 加载失败")
				} else {
					fileContent := string(buf)
					*AntiSpamReplaceKeywords = append(*AntiSpamReplaceKeywords, utils.SplitAndTrimSpace(fileContent, keywordsConf.FileSep)...)
				}
			}
		}

		// 关键词过滤
		handleContent := comment.Content
		replaced := false
		for _, keyword := range *AntiSpamReplaceKeywords {
			if strings.Contains(handleContent, keyword) {
				if keywordsConf.Pending {
					BlockCommentBy("关键词")
					break
				}

				if keywordsConf.ReplacTo != "" {
					handleContent = strings.Replace(handleContent, keyword, strings.Repeat(keywordsConf.ReplacTo, len([]rune(keyword))), -1)
					replaced = true
				}
			}
		}

		if !keywordsConf.Pending && replaced && keywordsConf.ReplacTo != "" {
			logrus.Info(fmt.Sprintf("[垃圾拦截] 关键词替换评论 ID=%d 原始内容=%s 替换内容=%s", comment.ID, strconv.Quote(comment.Content), strconv.Quote(handleContent)))

			// 保存评论
			comment.Content = handleContent
			query.UpdateComment(comment)
		}
	}
}
