package router

import (
	"net/http"
	"time"

	"pentagi/pkg/config"
	"pentagi/pkg/server/logger"
	"pentagi/pkg/server/models"
	"pentagi/pkg/server/rdb"
	"pentagi/pkg/server/response"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
)

func localUserRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.IsAborted() {
			return
		}

		session := sessions.Default(c)
		tid, ok := session.Get("tid").(string)

		if !ok || tid != models.UserTypeLocal.String() {
			response.Error(c, response.ErrLocalUserRequired, nil)
			return
		}

		c.Next()
	}
}

// autoLoginMiddleware transparently authenticates every request as the default
// admin user when AUTH_AUTO_LOGIN is enabled. It is intended for single-tenant
// deployments behind an external SSO gateway, where the internal PentAGI login
// page must never appear. It mirrors the session population done by the regular
// local-login flow (AuthService.AuthLogin) so the downstream auth middleware and
// the public /info endpoint see a normal admin session.
//
// When the flag is off (default) or a valid session already exists, it is a
// no-op, so self-hosted installs keep their normal authentication behavior.
func autoLoginMiddleware(db *gorm.DB, cfg *config.Config, sessionTimeout int) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.IsAborted() {
			return
		}

		if !cfg.AuthAutoLogin {
			c.Next()
			return
		}

		session := sessions.Default(c)

		// If a session already carries a user id, leave it untouched and let the
		// downstream auth middleware validate/expire it as usual.
		if session.Get("uid") != nil {
			c.Next()
			return
		}

		var user models.User
		if err := db.Take(&user, "mail = ?", cfg.AuthAutoLoginEmail).Error; err != nil {
			if !gorm.IsRecordNotFoundError(err) {
				logger.FromContext(c).WithError(err).Error("auto-login: failed to load default admin user")
				c.Next()
				return
			}

			// Seamless no-login deployment but the admin user isn't present yet: a
			// fresh database whose seed migration hasn't inserted it, or a custom
			// AUTH_AUTO_LOGIN_EMAIL that no seed row matches. Create an admin on
			// demand so the login page never blocks the user. This is idempotent
			// with the seed migration (mail is unique) and safe on every request
			// (only runs while no matching user exists).
			roleID := uint64(1) // 'Admin' role from the initial seed migration
			var adminRole models.Role
			if rerr := db.Table("roles").Where("name = ?", "Admin").Take(&adminRole).Error; rerr == nil && adminRole.ID != 0 {
				roleID = adminRole.ID
			}
			newUser := models.User{
				Mail:   cfg.AuthAutoLoginEmail,
				Name:   "admin",
				Type:   models.UserTypeLocal,
				Status: models.UserStatusActive,
				RoleID: roleID,
			}
			if cerr := db.Create(&newUser).Error; cerr != nil {
				logger.FromContext(c).WithError(cerr).Errorf("auto-login: failed to create default admin user '%s'", cfg.AuthAutoLoginEmail)
				c.Next()
				return
			}
			if lerr := db.Take(&user, "mail = ?", cfg.AuthAutoLoginEmail).Error; lerr != nil {
				logger.FromContext(c).WithError(lerr).Error("auto-login: failed to reload created admin user")
				c.Next()
				return
			}
			logger.FromContext(c).Infof("auto-login: created default admin user '%s' (role %d)", cfg.AuthAutoLoginEmail, roleID)
		}

		if user.Status != models.UserStatusActive {
			logger.FromContext(c).Errorf("auto-login: default admin user is not active (status '%s')", user.Status)
			c.Next()
			return
		}

		var privs []string
		if err := db.Table("privileges").Where("role_id = ?", user.RoleID).Pluck("name", &privs).Error; err != nil {
			logger.FromContext(c).WithError(err).Error("auto-login: failed to load admin privileges")
			c.Next()
			return
		}

		uuid, err := rdb.MakeUuidStrFromHash(user.Hash)
		if err != nil {
			logger.FromContext(c).WithError(err).Error("auto-login: failed to derive user uuid from hash")
			c.Next()
			return
		}

		now := time.Now()
		session.Set("uid", user.ID)
		session.Set("uhash", user.Hash)
		session.Set("rid", user.RoleID)
		session.Set("tid", models.UserTypeLocal.String())
		session.Set("prm", privs)
		session.Set("gtm", now.Unix())
		session.Set("exp", now.Add(time.Duration(sessionTimeout)*time.Second).Unix())
		session.Set("uuid", uuid)
		session.Set("uname", user.Name)
		session.Options(sessions.Options{
			HttpOnly: true,
			Secure:   c.Request.TLS != nil,
			SameSite: http.SameSiteLaxMode,
			Path:     baseURL,
			MaxAge:   sessionTimeout,
		})
		if err := session.Save(); err != nil {
			logger.FromContext(c).WithError(err).Error("auto-login: failed to save admin session")
			c.Next()
			return
		}

		c.Next()
	}
}

func noCacheMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate") // HTTP 1.1
		c.Header("Pragma", "no-cache")                                   // HTTP 1.0
		c.Header("Expires", "0")                                         // prevents caching at the proxy server
		c.Next()
	}
}
