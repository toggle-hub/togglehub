package apiutils

import (
	"strconv"

	"golang.org/x/crypto/bcrypt"
)

func GetPaginationParams(page, limit string) (int, int) {
	if page == "" {
		page = "1"
	}

	if limit == "" {
		limit = "10"
	}

	// Atoi can return an error if the string size is < 1
	// but because we check it before converting it'll probably
	// never happen
	pageNumber, _ := strconv.Atoi(page)
	limitNumber, _ := strconv.Atoi(limit)

	return pageNumber, limitNumber
}

func EncryptPassword(password string) (string, error) {
	encryptedPassword, err := bcrypt.GenerateFromPassword([]byte(password), 8)
	if err != nil {
		return "", err
	}

	return string(encryptedPassword), nil
}
