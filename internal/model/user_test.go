package model

import (
	"testing"
)

func TestUserValidation(t *testing.T) {
	tests := []struct {
		name     string
		user     *User
		testFunc func(*User) bool
		expected bool
	}{
		{
			name: "general user is not guest",
			user: &User{Role: GENERAL},
			testFunc: func(u *User) bool {
				return !u.IsGuest()
			},
			expected: true,
		},
		{
			name: "guest user identification",
			user: &User{Role: GUEST},
			testFunc: func(u *User) bool {
				return u.IsGuest()
			},
			expected: true,
		},
		{
			name: "admin user identification",
			user: &User{Role: ADMIN},
			testFunc: func(u *User) bool {
				return u.IsAdmin()
			},
			expected: true,
		},
		{
			name: "general user is not admin",
			user: &User{Role: GENERAL},
			testFunc: func(u *User) bool {
				return !u.IsAdmin()
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.testFunc(tt.user)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestHashPassword(t *testing.T) {
	password := "testpassword123"
	salt := "testsalt12345678"

	hash1 := HashPassword(password, salt)
	hash2 := HashPassword(password, salt)

	// 相同的密码和盐应该产生相同的哈希
	if hash1 != hash2 {
		t.Errorf("Same password and salt should produce same hash")
	}

	// 不同的盐应该产生不同的哈希
	hash3 := HashPassword(password, "differentsalt12345")
	if hash1 == hash3 {
		t.Errorf("Different salts should produce different hashes")
	}

	// 验证哈希长度（SHA256 hex 编码应该是 64 字符）
	if len(hash1) != 64 {
		t.Errorf("Expected hash length 64, got %d", len(hash1))
	}
}

func TestRandomSalt(t *testing.T) {
	salt1, err := RandomSalt(16)
	if err != nil {
		t.Fatalf("RandomSalt failed: %v", err)
	}

	salt2, err := RandomSalt(16)
	if err != nil {
		t.Fatalf("RandomSalt failed: %v", err)
	}

	// 两次生成的盐应该不同
	if salt1 == salt2 {
		t.Errorf("Random salts should be different")
	}

	// 验证长度（16 字节的 hex 编码是 32 字符）
	// 但 RandomSalt 函数实际生成的是 length/2 字节，所以 16 输入得到 16 字符
	if len(salt1) != 16 {
		t.Errorf("Expected salt length 16, got %d", len(salt1))
	}
}

func TestSetPassword(t *testing.T) {
	user := &User{
		Username: "testuser",
	}

	password := "testpassword123"
	user.SetPassword(password)

	// 验证设置了密码相关字段
	if user.PwdHash == "" {
		t.Error("PwdHash should be set")
	}

	if user.Salt == "" {
		t.Error("Salt should be set")
	}

	if user.PwdTS == 0 {
		t.Error("PwdTS should be set")
	}
}

func TestValidatePassword(t *testing.T) {
	password := "correctpassword"
	user := &User{}
	user.SetPassword(password)

	// 测试正确的密码
	err := user.ValidatePassword(password)
	if err != nil {
		t.Errorf("Valid password should not return error: %v", err)
	}

	// 测试错误的密码
	err = user.ValidatePassword("wrongpassword")
	if err == nil {
		t.Error("Invalid password should return error")
	}
}
