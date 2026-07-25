package app

import (
	"errors"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"

	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
)

func getWorkflowUserByID(id int, selectAll bool) (*identityschema.User, error) {
	if id <= 0 {
		return nil, errors.New("id 为空！")
	}

	user := &identityschema.User{Id: id}
	query := platformdb.DB
	if !selectAll {
		query = query.Omit("password")
	}
	if err := query.First(user, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return user, nil
}

func listWorkflowUserBaseByIDs(userIDs []int) (map[int]*identityschema.UserBase, error) {
	result := make(map[int]*identityschema.UserBase, len(userIDs))
	if len(userIDs) == 0 {
		return result, nil
	}

	users := make([]identityschema.User, 0, len(userIDs))
	if err := platformdb.DB.Where("id IN ?", userIDs).
		Find(&users).Error; err != nil {
		return nil, err
	}

	for _, user := range users {
		userCopy := user
		result[user.Id] = userCopy.ToBaseUser()
	}
	return result, nil
}
