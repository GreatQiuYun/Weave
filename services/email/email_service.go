package email

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"weave/models"
	"weave/pkg"

	"gorm.io/gorm"
)

// EmailConfig 邮件服务器配置
type EmailConfig struct {
	SMTPServer string
	SMTPPort   int
	Username   string
	Password   string
	From       string
}

// EmailService 邮件服务
type EmailService struct {
	config EmailConfig
}

// NewEmailService 创建新的邮件服务实例
func NewEmailService(config EmailConfig) *EmailService {
	return &EmailService{config: config}
}

// GenerateVerificationCode 生成6位数字验证码
func (s *EmailService) GenerateVerificationCode() (string, error) {
	result := make([]string, 6)
	for i := 0; i < 6; i++ {
		// 生成0-9之间的随机数
		num, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		result[i] = strconv.Itoa(int(num.Int64()))
	}
	return strings.Join(result, ""), nil
}

// SendVerificationCode 发送验证码到指定邮箱
func (s *EmailService) SendVerificationCode(email, code string) error {
	subject := "Weave 登录验证码"
	body := fmt.Sprintf(`<html>
<body style="font-family: Arial, sans-serif;">
<h2>您的验证码</h2>
<p>尊敬的用户：</p>
<p>您正在登录系统，验证码为：</p>
<div style="font-size: 24px; font-weight: bold; color: #007bff; padding: 10px 0;">%s</div>
<p>该验证码有效期为5分钟，请尽快使用。</p>
<p>请勿将验证码泄露给他人。</p>
<p>如果您没有尝试登录，请忽略此邮件。</p>
<p>此致<br>Weave</p>
</body>
</html>`, code)

	return s.SendEmail(email, subject, body)
}

// SendEmail 发送邮件
func (s *EmailService) SendEmail(to, subject, body string) error {
	// 构建邮件头
	header := make(map[string]string)
	header["From"] = s.config.From
	header["To"] = to
	header["Subject"] = "=?UTF-8?B?" + s.base64Encode(subject) + "?="
	header["MIME-Version"] = "1.0"
	header["Content-Type"] = "text/html; charset=UTF-8"

	// 构建邮件内容
	message := ""
	for k, v := range header {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	message += "\r\n" + body

	// 连接SMTP服务器
	auth := smtp.PlainAuth("", s.config.Username, s.config.Password, s.config.SMTPServer)
	serverAddr := fmt.Sprintf("%s:%d", s.config.SMTPServer, s.config.SMTPPort)

	return smtp.SendMail(serverAddr, auth, s.config.From, []string{to}, []byte(message))
}

// base64Encode Base64编码（简化版）
func (s *EmailService) base64Encode(input string) string {
	const base64Chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	var result strings.Builder
	data := []byte(input)
	n := len(data)
	for i := 0; i < n; i += 3 {
		triplet := make([]byte, 3)
		for j := 0; j < 3 && i+j < n; j++ {
			triplet[j] = data[i+j]
		}
		a := uint(triplet[0]) << 16
		b := uint(triplet[1]) << 8
		c := uint(triplet[2])
		total := a | b | c
		result.WriteByte(base64Chars[(total>>18)&0x3F])
		result.WriteByte(base64Chars[(total>>12)&0x3F])
		if i+1 < n {
			result.WriteByte(base64Chars[(total>>6)&0x3F])
		} else {
			result.WriteByte('=')
		}
		if i+2 < n {
			result.WriteByte(base64Chars[total&0x3F])
		} else {
			result.WriteByte('=')
		}
	}
	return result.String()
}

// CreateVerificationCode 创建并保存验证码记录
func (s *EmailService) CreateVerificationCode(email string, tenantID uint) (*models.EmailVerificationCode, error) {
	// 生成验证码
	code, err := s.GenerateVerificationCode()
	if err != nil {
		return nil, err
	}

	// 创建验证码记录
	verificationCode := &models.EmailVerificationCode{
		Email:     email,
		Code:      code,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(5 * time.Minute), // 5分钟有效期
		Used:      false,
		TenantID:  tenantID,
	}

	return verificationCode, nil
}

// VerifyCode 验证邮箱验证码
func (s *EmailService) VerifyCode(email, code string, tenantID uint) (bool, error) {
	// 查找最新的未使用且未过期的验证码
	var verificationCode models.EmailVerificationCode
	result := pkg.DB.Where("email = ? AND code = ? AND used = false AND expires_at > ? AND tenant_id = ?",
		email, code, time.Now(), tenantID).Order("created_at DESC").First(&verificationCode)

	if result.Error != nil {
		return false, result.Error
	}

	// 标记验证码为已使用
	verificationCode.Used = true
	if err := pkg.DB.Save(&verificationCode).Error; err != nil {
		return false, err
	}

	return true, nil
}

// GetLastVerificationTime 获取用户最近一次获取验证码的时间
func (s *EmailService) GetLastVerificationTime(email string, tenantID uint) (time.Time, error) {
	var verificationCode models.EmailVerificationCode
	result := pkg.DB.Where("email = ? AND tenant_id = ?", email, tenantID).Order("created_at DESC").First(&verificationCode)

	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			// 如果没有找到记录，返回零时间
			return time.Time{}, nil
		}
		return time.Time{}, result.Error
	}

	return verificationCode.CreatedAt, nil
}

// CheckRateLimit 检查获取验证码的频率限制（防止滥用）
func (s *EmailService) CheckRateLimit(email string, tenantID uint) (bool, error) {
	lastTime, err := s.GetLastVerificationTime(email, tenantID)
	if err != nil {
		return false, err
	}

	// 如果距离上次发送验证码不足60秒，则限制再次发送
	if !lastTime.IsZero() && time.Since(lastTime) < 60*time.Second {
		return false, nil
	}

	return true, nil
}
