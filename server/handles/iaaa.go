package handles

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"pkuphysu-backend/internal/db"
	"pkuphysu-backend/internal/model"
	"pkuphysu-backend/internal/utils"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

type IaaaLoginReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type IaaaResponse struct {
	Success bool   `json:"success"`
	Msg     string `json:"msg"`
	Token   string `json:"token,omitempty"`
}

type PortalUserResponse struct {
	Success  bool   `json:"success"`
	UserId   string `json:"userId"`
	UserName string `json:"userName"`
}

func generateRandomString() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%f", float64(n.Int64())/1000000000.0), nil
}

func IaaaLogin(c *gin.Context) {
	var req IaaaLoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondError(c, http.StatusBadRequest, "invalid_request", err)
		return
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		utils.RespondError(c, http.StatusInternalServerError, "cookie_jar_failed", err)
		return
	}
	client := &http.Client{
		Jar: jar,
	}

	formData := url.Values{}
	formData.Set("appid", "portal2017")
	formData.Set("userName", req.Username)
	formData.Set("password", req.Password)
	formData.Set("randCode", "")
	formData.Set("smsCode", "")
	formData.Set("otpCode", "")
	formData.Set("remTrustChk", "false")
	formData.Set("redirUrl", "https://portal.pku.edu.cn/portal2017/ssoLogin.do")

	resp, err := client.PostForm("https://iaaa.pku.edu.cn/iaaa/oauthlogin.do", formData)
	if err != nil {
		log.Errorf("Failed to send IAAA login request: %v", err)
		utils.RespondError(c, http.StatusInternalServerError, "iaaa_request_failed", err)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("Failed to read IAAA response body: %v", err)
		utils.RespondError(c, http.StatusInternalServerError, "iaaa_response_read_failed", err)
		return
	}

	var iaaaResp IaaaResponse
	if err := json.Unmarshal(body, &iaaaResp); err != nil {
		utils.RespondError(c, http.StatusInternalServerError, "iaaa_response_parse_failed", fmt.Errorf("failed to parse IAAA response: %s", string(body)))
		return
	}

	if !iaaaResp.Success {
		log.Warnf("IAAA login failed for user %s: %s", req.Username, iaaaResp.Msg)
		utils.RespondError(c, http.StatusUnauthorized, "invalid_credentials", fmt.Errorf("IAAA login failed: %s", iaaaResp.Msg))
		return
	}

	if iaaaResp.Token == "" {
		utils.RespondError(c, http.StatusInternalServerError, "iaaa_token_missing", fmt.Errorf("IAAA token is missing in response"))
		return
	}

	randStr, err := generateRandomString()
	if err != nil {
		utils.RespondError(c, http.StatusInternalServerError, "random_generation_failed", err)
		return
	}

	ssoParams := url.Values{}
	ssoParams.Set("_rand", randStr)
	ssoParams.Set("token", iaaaResp.Token)

	ssoUrl := "https://portal.pku.edu.cn/portal2017/ssoLogin.do?" + ssoParams.Encode()
	ssoResp, err := client.Get(ssoUrl)
	if err != nil {
		log.Errorf("Failed to send SSO request: %v", err)
		utils.RespondError(c, http.StatusInternalServerError, "portal_sso_failed", err)
		return
	}
	ssoResp.Body.Close()

	isUserLoggedResp, err := client.Post("https://portal.pku.edu.cn/portal2017/isUserLogged.do", "application/x-www-form-urlencoded", nil)
	if err != nil {
		log.Errorf("Failed to get user info from portal: %v", err)
		utils.RespondError(c, http.StatusInternalServerError, "portal_user_info_failed", err)
		return
	}
	defer isUserLoggedResp.Body.Close()

	userInfoBody, err := io.ReadAll(isUserLoggedResp.Body)
	if err != nil {
		log.Errorf("Failed to read user info response: %v", err)
		utils.RespondError(c, http.StatusInternalServerError, "user_info_read_failed", err)
		return
	}

	var portalUser PortalUserResponse
	if err := json.Unmarshal(userInfoBody, &portalUser); err != nil {
		utils.RespondError(c, http.StatusInternalServerError, "user_info_parse_failed", fmt.Errorf("failed to parse user info: %s", string(userInfoBody)))
		return
	}

	var user *model.User
	user, err = db.GetUserByStuid(portalUser.UserId)
	if err != nil {
		user = &model.User{
			Username: portalUser.UserName,
			Stuname:  portalUser.UserName,
			Stuid:    portalUser.UserId,
			Role:     model.GENERAL,
			Verified: true,
		}

		if err := db.CreateUser(user); err != nil {
			utils.RespondError(c, http.StatusInternalServerError, "user_creation_failed", err)
			return
		}
	} else {
		user.Stuname = portalUser.UserName
		if err := db.UpdateUser(user); err != nil {
			utils.RespondError(c, http.StatusInternalServerError, "user_update_failed", err)
			return
		}
	}

	token, err := utils.GenerateToken(user)
	if err != nil {
		utils.RespondError(c, http.StatusInternalServerError, "token_generation_failed", err)
		return
	}

	utils.RespondSuccess(c, gin.H{"token": token, "username": user.Username, "userid": user.ID})
}
