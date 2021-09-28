package http

import (
	"net/http"
	"strings"

	"github.com/ArtalkJS/ArtalkGo/config"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/markbates/pkger"
	echolog "github.com/onrik/logrus/echo"
	"github.com/sirupsen/logrus"
)

type Map = map[string]interface{}

func Run() {
	e := echo.New()
	e.HideBanner = true

	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: config.Instance.AllowOrigin,
	}))

	// Logger
	e.Logger = echolog.NewLogger(logrus.StandardLogger(), "")
	logConf := echolog.DefaultConfig
	logConf.Fields = []string{"ip", "latency", "status", "referer", "user_agent"} // "headers"
	e.Use(echolog.Middleware(logConf))

	// Action Limit
	ActionPermissionConf := ActionPermissionConf{
		Skipper: func(c echo.Context) bool {
			// 不启用操作限制的 path
			skipPath := []string{
				"/api/captcha/",
				"/api/get", // 获取评论不做限制
			}

			for _, p := range skipPath {
				if strings.HasPrefix(c.Request().URL.Path, p) {
					return true
				}
			}

			return false
		},
	}
	e.Use(ActionPermission(ActionPermissionConf))

	CommonJwtConfig = middleware.JWTConfig{
		Claims:        &jwtCustomClaims{},
		ContextKey:    "user",
		SigningMethod: "HS256",
		TokenLookup:   "header:Authorization,query:token,form:token",
		AuthScheme:    "Bearer",
		SigningKey:    []byte(config.Instance.AppKey),
	}

	// Route
	InitRoute(e)

	e.Logger.Fatal(e.Start(config.Instance.HttpAddr))
}

func InitRoute(e *echo.Echo) {
	f, err := pkger.Open("/frontend")
	if err != nil {
		logrus.Fatal(err)
		return
	}

	fileServer := http.FileServer(f)
	e.GET("/*", echo.WrapHandler(fileServer))

	// api
	api := e.Group("/api")

	api.POST("/add", ActionAdd)
	api.POST("/get", ActionGet)
	api.POST("/user-get", ActionUserGet)
	api.GET("/login", ActionLogin)
	api.POST("/login", ActionLogin)

	// api/captcha
	ca := api.Group("/captcha")
	ca.GET("/refresh", ActionCaptchaGet)
	ca.GET("/check", ActionCaptchaCheck)

	// api/admin
	admin := api.Group("/admin", middleware.JWTWithConfig(CommonJwtConfig)) // use jwt
	admin.POST("/page-edit", ActionAdminPageEdit)
	admin.POST("/page-del", ActionAdminPageDel)
	admin.POST("/comment-edit", ActionAdminCommentEdit)
	admin.POST("/comment-del", ActionAdminCommentDel)
	admin.POST("/page-fetch", ActionAdminPageFetch)
	admin.POST("/send-mail", ActionAdminSendMail)
	admin.POST("/site-add", ActionAdminSiteAdd)
	admin.POST("/site-del", ActionAdminSiteDel)
}