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

		// Diagnostic breadcrumb surfaced on /info (auto_login_status) so a guest
		// response is self-explaining without pod logs. Overwritten below as the
		// flow progresses; "attempted" means the middleware ran but never authed.
		c.Set("autoLoginStatus", "attempted")

		var user models.User
		if err := db.Take(&user, "mail = ?", cfg.AuthAutoLoginEmail).Error; err != nil {
			if !gorm.IsRecordNotFoundError(err) {
				c.Set("autoLoginStatus", "load_error: "+err.Error())
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
				c.Set("autoLoginStatus", "create_error: "+cerr.Error())
				logger.FromContext(c).WithError(cerr).Errorf("auto-login: failed to create default admin user '%s'", cfg.AuthAutoLoginEmail)
				c.Next()
				return
			}
			if lerr := db.Take(&user, "mail = ?", cfg.AuthAutoLoginEmail).Error; lerr != nil {
				c.Set("autoLoginStatus", "reload_error: "+lerr.Error())
				logger.FromContext(c).WithError(lerr).Error("auto-login: failed to reload created admin user")
				c.Next()
				return
			}
			c.Set("autoLoginStatus", "created")
			logger.FromContext(c).Infof("auto-login: created default admin user '%s' (role %d)", cfg.AuthAutoLoginEmail, roleID)
		}

		if user.Status != models.UserStatusActive {
			c.Set("autoLoginStatus", "inactive: "+string(user.Status))
			logger.FromContext(c).Errorf("auto-login: default admin user is not active (status '%s')", user.Status)
			c.Next()
			return
		}

		// Leave a genuinely valid session alone (avoids re-saving the cookie on
		// every request). "Valid" means it authenticates THIS admin — matching uid
		// AND hash — and is unexpired. A cookie that merely has a uid is not enough:
		// a stale cookie from a previous install carries a uid + a future exp but a
		// hash that no longer matches, and the downstream auth check would reject it
		// as guest. In that case we must re-mint rather than skip.
		if sessionMatchesUser(session, user) && sessionUnexpired(session) {
			c.Set("autoLoginStatus", "session_ok")
			c.Next()
			return
		}

		var privs []string
		if err := db.Table("privileges").Where("role_id = ?", user.RoleID).Pluck("name", &privs).Error; err != nil {
			c.Set("autoLoginStatus", "privs_error: "+err.Error())
			logger.FromContext(c).WithError(err).Error("auto-login: failed to load admin privileges")
			c.Next()
			return
		}

		uuid, err := rdb.MakeUuidStrFromHash(user.Hash)
		if err != nil {
			c.Set("autoLoginStatus", "uuid_error: "+err.Error())
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
			c.Set("autoLoginStatus", "save_error: "+err.Error())
			logger.FromContext(c).WithError(err).Error("auto-login: failed to save admin session")
			c.Next()
			return
		}

		c.Set("autoLoginStatus", "ok")
		c.Next()
	}
}

// sessionMatchesUser reports whether the session already authenticates exactly
// the given user (both the id and the hash match). A stale cookie from an
// earlier install keeps a uid but its hash no longer matches the current admin,
// so this returns false and the caller re-mints the session.
func sessionMatchesUser(session sessions.Session, user models.User) bool {
	uid, ok := session.Get("uid").(uint64)
	if !ok || uid != user.ID {
		return false
	}
	uhash, ok := session.Get("uhash").(string)
	return ok && uhash == user.Hash
}

// sessionUnexpired reports whether the session carries an "exp" (unix seconds)
// that is still in the future. Used by autoLoginMiddleware to tell a live
// session from a stale cookie left over from a previous deployment/install.
func sessionUnexpired(session sessions.Session) bool {
	exp := session.Get("exp")
	if exp == nil {
		return false
	}

	var expUnix int64
	switch v := exp.(type) {
	case int64:
		expUnix = v
	case int:
		expUnix = int64(v)
	case float64:
		expUnix = int64(v)
	default:
		return false
	}

	return expUnix > time.Now().Unix()
}

func noCacheMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate") // HTTP 1.1
		c.Header("Pragma", "no-cache")                                   // HTTP 1.0
		c.Header("Expires", "0")                                         // prevents caching at the proxy server
		c.Next()
	}
}
